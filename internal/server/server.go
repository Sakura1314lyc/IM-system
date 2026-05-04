package server

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"IM-system/internal/config"
	"IM-system/internal/db"
)

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

type Server struct {
	Ip   string
	Port int

	OnlineMap map[string]*User
	mapLock   sync.RWMutex

	WSConns map[string]*WSClient
	wsLock  sync.RWMutex

	Sessions SessionStore

	DB Storage

	loginLimiter    *rateLimiter
	registerLimiter *rateLimiter
	sendLimiter     *rateLimiter

	TLSConfig *tls.Config
	ctx       context.Context
	cancel    context.CancelFunc

	tcpListener net.Listener
	webListener net.Listener
	wg          sync.WaitGroup

	idleTimeout    time.Duration
	sessionTTL     time.Duration
	sessionCleanup time.Duration
	maxMsgLength   int
	uploadDir      string
	webAddr        string
	historyLimit   int
	historyMax     int
	serverID       string
	bus            MessageBus
}

func New(cfg *config.Config) (*Server, error) {
	database, err := db.InitDatabase(cfg.DB.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{
		Ip:              cfg.Server.IP,
		Port:            cfg.Server.Port,
		OnlineMap:       make(map[string]*User),
		WSConns:         make(map[string]*WSClient),
		DB:              database,
		loginLimiter:    newRateLimiter(cfg.App.RateLimit, cfg.App.RateWindow.ToDuration()),
		registerLimiter: newRateLimiter(cfg.App.RateLimit, cfg.App.RateWindow.ToDuration()),
		sendLimiter:     newRateLimiter(cfg.App.RateLimit, cfg.App.RateWindow.ToDuration()),
		ctx:             ctx,
		cancel:          cancel,
		idleTimeout:     cfg.Server.IdleTimeout.ToDuration(),
		sessionTTL:      cfg.App.SessionTTL.ToDuration(),
		sessionCleanup:  cfg.App.SessionCleanup.ToDuration(),
		maxMsgLength:    cfg.App.MaxMsgLength,
		webAddr:         cfg.Web.Addr,
		uploadDir:       cfg.Web.UploadDir,
		historyLimit:    cfg.App.HistoryLimit,
		historyMax:      cfg.App.HistoryMax,
	}

		// Generate server ID for bus message deduplication
		rawID := make([]byte, 8)
		rand.Read(rawID)
		server.serverID = hex.EncodeToString(rawID)

	if cfg.App.SessionBackend == "redis" {
		store, err := NewRedisSessionStore(cfg.App.RedisAddr, cfg.App.RedisPassword, cfg.App.RedisDB)
		if err != nil {
			return nil, fmt.Errorf("redis session store: %w", err)
		}
		server.Sessions = store
		slog.Info("using redis session store", "addr", cfg.App.RedisAddr)

		bus, err := NewRedisMessageBus(server.serverID, cfg.App.RedisAddr, cfg.App.RedisPassword, cfg.App.RedisDB)
		if err != nil {
			return nil, fmt.Errorf("redis message bus: %w", err)
		}
		server.bus = bus
		slog.Info("using redis message bus", "addr", cfg.App.RedisAddr)
		server.wg.Add(1)
		go server.startBusReceiver()
	} else {
		server.Sessions = NewMemorySessionStore()
		server.wg.Add(1)
		go server.cleanupSessions()
	}

	if cfg.Server.TLS {
		cert, err := generateSelfSignedCert()
		if err != nil {
			return nil, fmt.Errorf("failed to generate TLS cert: %w", err)
		}
		server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		slog.Info("tls enabled")
	}

	return server, nil
}

func (s *Server) Start() {
	defer s.wg.Done()

	var listener net.Listener
	var err error

	if s.TLSConfig != nil {
		listener, err = tls.Listen("tcp", fmt.Sprintf("%s:%d", s.Ip, s.Port), s.TLSConfig)
		s.tcpListener = listener
		if err != nil {
			slog.Error("tls listen failed", "error", err)
			return
		}
		slog.Info("tcp server listening with tls", "addr", fmt.Sprintf("%s:%d", s.Ip, s.Port))
	} else {
		var addr string
		var listenErr error
		listener, addr, listenErr = listenWithFallback(fmt.Sprintf("%s:%d", s.Ip, s.Port), 20)
		s.tcpListener = listener
		err = listenErr
		if err != nil {
			slog.Error("listen failed", "error", err)
			return
		}
		if _, p, splitErr := net.SplitHostPort(addr); splitErr == nil {
			var parsedPort int
			fmt.Sscanf(p, "%d", &parsedPort)
			if parsedPort > 0 {
				s.Port = parsedPort
			}
		}
		slog.Info("tcp server listening", "addr", addr)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				slog.Error("accept failed", "error", err)
				continue
			}
		}
		go s.Handler(conn)
	}
}

// WGAdd increments the server's WaitGroup counter (safe for external callers).
func (s *Server) WGAdd(n int) {
	s.wg.Add(n)
}

func (s *Server) Shutdown() {
	s.cancel()
	if s.webListener != nil {
		s.webListener.Close()
	}
	if s.tcpListener != nil {
		s.tcpListener.Close()
	}
	s.Sessions.Close()
	if s.bus != nil {
		s.bus.Close()
	}
	s.DB.Close()
	s.wg.Wait()
}

func (s *Server) Handler(conn net.Conn) {
	user := NewUser(conn, s)
	user.Online()

	isLive := make(chan bool, 1)
	var offlineOnce sync.Once

	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			msg := strings.TrimSpace(scanner.Text())
			if msg == "" {
				continue
			}

			user.DoMessage(msg)
			select {
			case isLive <- true:
			default:
			}
		}
		if err := scanner.Err(); err != nil {
			slog.Error("handler scanner err", "error", err)
		}
		offlineOnce.Do(func() { user.Offline() })
	}()

	for {
		select {
		case <-isLive:
		case <-time.After(s.idleTimeout):
			slog.Warn("user idle timeout, kicked", "name", user.Name)
			user.SendMsg("你已被踢出")
			offlineOnce.Do(func() { user.Offline() })
			return
		}
	}
}

func (s *Server) BroadCast(user *User, msg string) {
	msg = sanitizeInputWithLimit(msg, s.maxMsgLength)
	sendMsg := user.Name + ": " + msg
	slog.Debug("broadcast from tcp", "from", user.Name, "addr", user.Addr, "msg", msg)

	fromUser, err := s.DB.GetUserByUsername(user.Name)
	if err == nil {
		_, err = s.DB.SaveMessage(fromUser.ID, nil, nil, msg, "public")
		if err != nil {
			slog.Warn("failed to save public message", "error", err)
		}
	}

	// Deliver to TCP clients
	s.fanoutTCP(sendMsg, user.Name)

	// Deliver to WebSocket clients with proper JSON format
	isSystem := msg == "已上线" || msg == "已下线"
	s.wsLock.RLock()
	for _, client := range s.WSConns {
		if isSystem {
			s.writeWS(client, wsOutgoing{
				Type:    "system",
				Content: user.Name + " " + msg,
			})
		} else {
			s.writeWS(client, wsOutgoing{
				Type:    "message",
				Mode:    "public",
				From:    user.Name,
				Avatar:  user.Avatar,
				Content: msg,
				Time:    nowLabelWS(),
			})
		}
	}
	s.wsLock.RUnlock()
	if s.bus != nil {
		isSystem := msg == "已上线" || msg == "已下线"
		if isSystem {
			s.bus.Publish(BusMessage{
				Type: "system", From: user.Name, Content: user.Name + " " + msg,
			})
		} else {
			s.bus.Publish(BusMessage{
				Type: "public", From: user.Name, Avatar: user.Avatar,
				Content: msg, Time: nowLabelWS(),
			})
		}
	}

}

func (s *Server) BroadCastFromWeb(name string, msg string, avatar string) {
	msg = sanitizeInputWithLimit(msg, s.maxMsgLength)
	sendMsg := name + ": " + msg
	slog.Debug("broadcast from web", "from", name, "msg", msg)

	if fromUser, err := s.DB.GetUserByUsername(name); err == nil {
		if _, err := s.DB.SaveMessage(fromUser.ID, nil, nil, msg, "public"); err != nil {
			slog.Warn("failed to save web public message", "error", err)
		}
	}

	s.fanoutTCP(sendMsg, name)

	s.wsLock.RLock()
	for _, client := range s.WSConns {
		s.writeWS(client, wsOutgoing{
			Type:    "message",
			Mode:    "public",
			From:    name,
			Avatar:  avatar,
			Content: msg,
			Time:    nowLabelWS(),
		})
	}
	s.wsLock.RUnlock()
	if s.bus != nil {
		s.bus.Publish(BusMessage{
			Type: "public", From: name, Avatar: avatar,
			Content: msg, Time: nowLabelWS(),
		})
	}

}

func (s *Server) fanoutTCP(msg string, from string) {
	s.mapLock.RLock()
	for _, cli := range s.OnlineMap {
		select {
		case cli.C <- msg:
		default:
			slog.Warn("user tcp queue full, dropped", "name", cli.Name, "from", from)
		}
	}
	s.mapLock.RUnlock()
}
func (s *Server) BroadCastToGroup(groupName, from, msg, avatar string) error {
	msg = sanitizeInputWithLimit(msg, s.maxMsgLength)

	fromUser, err := s.DB.GetUserByUsername(from)
	if err != nil {
		return fmt.Errorf("发送者不存在: %v", err)
	}

	members, err := s.DB.GetGroupMembers(groupName)
	if err != nil {
		return fmt.Errorf("获取群组成员失败: %v", err)
	}

	if len(members) == 0 {
		return fmt.Errorf("群组不存在或没有成员")
	}

	group, err := s.DB.GetGroupByName(groupName)
	if err != nil {
		return fmt.Errorf("获取群组ID失败: %v", err)
	}
	groupID := group.ID

	sendMsg := fmt.Sprintf("[群聊 %s] %s: %s", groupName, from, msg)
	slog.Debug("group broadcast", "group", groupName, "from", from, "msg", msg)

	_, err = s.DB.SaveMessage(fromUser.ID, nil, &groupID, msg, "group")
	if err != nil {
		slog.Warn("failed to save group message", "error", err)
	}

	for _, member := range members {
		s.mapLock.RLock()
		if user, ok := s.OnlineMap[member.Username]; ok {
			user.SendMsg(sendMsg + "\n")
		}
		s.mapLock.RUnlock()

		outgoing := wsOutgoing{
			Type:    "message",
			Mode:    "group",
			Group:   groupName,
			From:    from,
			Avatar:  avatar,
			Content: msg,
			Time:    nowLabelWS(),
		}
		s.wsLock.RLock()
		client, ok := s.WSConns[member.Username]
		s.wsLock.RUnlock()
		if ok {
			s.writeWS(client, outgoing)
		}
	}
	return nil
}

func (s *Server) SendPrivate(from, to, content, avatar string) error {
	content = sanitizeInputWithLimit(content, s.maxMsgLength)

	fromUser, err := s.DB.GetUserByUsername(from)
	if err != nil {
		return fmt.Errorf("发送者不存在: %v", err)
	}

	toUser, err := s.DB.GetUserByUsername(to)
	if err != nil {
		return fmt.Errorf("接收者不存在: %v", err)
	}

	_, err = s.DB.SaveMessage(fromUser.ID, &toUser.ID, nil, content, "private")
	if err != nil {
		slog.Warn("failed to save private message", "error", err)
	}

	s.mapLock.RLock()
	if user, ok := s.OnlineMap[to]; ok {
		user.SendMsg("[私聊] " + from + ": " + content + "\n")
		s.mapLock.RUnlock()
		return nil
	}
	s.mapLock.RUnlock()

	outgoing := wsOutgoing{
		Type:    "message",
		Mode:    "private",
		From:    from,
		To:      to,
		Avatar:  avatar,
		Content: content,
		Time:    nowLabelWS(),
	}
	s.wsLock.RLock()
	client, ok := s.WSConns[to]
	s.wsLock.RUnlock()
	if ok {
		s.writeWS(client, outgoing)
		return nil
	}

	slog.Debug("private message saved for offline user", "from", from, "to", to)
	return nil
}

func (s *Server) GetOnlineUsers() []map[string]string {
	seen := make(map[string]bool)
	users := make([]map[string]string, 0)

	s.mapLock.RLock()
	for _, u := range s.OnlineMap {
		if !seen[u.Name] {
			seen[u.Name] = true
			users = append(users, map[string]string{
				"name":   u.Name,
				"avatar": u.Avatar,
			})
		}
	}
	s.mapLock.RUnlock()

	s.wsLock.RLock()
	for name, client := range s.WSConns {
		if !seen[name] {
			seen[name] = true
			users = append(users, map[string]string{
				"name":   name,
				"avatar": client.Avatar,
			})
		}
	}
	s.wsLock.RUnlock()

	return users
}

func (s *Server) IsNameTaken(name string) bool {
	s.mapLock.RLock()
	_, inTcp := s.OnlineMap[name]
	s.mapLock.RUnlock()

	if inTcp {
		return true
	}

	s.wsLock.RLock()
	_, inWeb := s.WSConns[name]
	s.wsLock.RUnlock()

	return inWeb
}

func (s *Server) RenameUser(oldName, newName string) bool {
	s.mapLock.Lock()
	defer s.mapLock.Unlock()

	if _, exists := s.OnlineMap[newName]; exists {
		return false
	}

	user, ok := s.OnlineMap[oldName]
	if !ok {
		return false
	}

	delete(s.OnlineMap, oldName)
	s.OnlineMap[newName] = user

	return true
}

func (s *Server) RenameWSClient(oldName, newName string) error {
	s.wsLock.Lock()
	defer s.wsLock.Unlock()

	if _, exists := s.WSConns[newName]; exists {
		return fmt.Errorf("用户名已被占用")
	}

	client, ok := s.WSConns[oldName]
	if !ok {
		return fmt.Errorf("旧用户名不存在")
	}

	delete(s.WSConns, oldName)
	client.Name = newName
	s.WSConns[newName] = client
	return nil
}

func (s *Server) Authenticate(username, password string) (string, bool) {
	user, err := s.DB.AuthenticateUser(username, password)
	if err != nil {
		return "", false
	}
	return user.Avatar, true
}

func (s *Server) Register(username, password, avatar string) error {
	return s.DB.RegisterUser(username, password, avatar)
}

func (s *Server) UpdateAvatar(username, avatar string) (string, error) {
	path, err := saveAvatarFile(s.uploadDir, username, avatar)
	if err != nil {
		return "", err
	}
	return path, s.DB.UpdateUserAvatar(username, path)
}

func (s *Server) CreateGroup(groupName, creator string) error {
	creatorUser, err := s.DB.GetUserByUsername(creator)
	if err != nil {
		return fmt.Errorf("用户不存在: %v", err)
	}
	_, err = s.DB.CreateGroup(groupName, creatorUser.ID, "")
	return err
}

func (s *Server) JoinGroup(groupName, user string) error {
	userObj, err := s.DB.GetUserByUsername(user)
	if err != nil {
		return fmt.Errorf("用户不存在: %v", err)
	}
	return s.DB.JoinGroup(groupName, userObj.ID)
}

func (s *Server) LeaveGroup(groupName, user string) error {
	userObj, err := s.DB.GetUserByUsername(user)
	if err != nil {
		return fmt.Errorf("用户不存在: %v", err)
	}
	return s.DB.LeaveGroup(groupName, userObj.ID)
}

func (s *Server) CreateSession(username string) (string, error) {
	return s.Sessions.Create(username, s.sessionTTL)
}

func (s *Server) GetUsernameByToken(token string) (string, bool) {
	return s.Sessions.GetUsername(token)
}

func (s *Server) UpdateSessionUsername(token, username string) {
	s.Sessions.UpdateUsername(token, username)
}

func (s *Server) DeleteSession(token string) {
	s.Sessions.Delete(token)
}

func (s *Server) cleanupSessions() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.sessionCleanup)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.Sessions.Cleanup(time.Now())
		}
	}
}
func (s *Server) startBusReceiver() {
	defer s.wg.Done()
	for {
		msg, err := s.bus.Receive(s.ctx)
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				slog.Error("bus receive error", "error", err)
				continue
			}
		}
		if msg.OriginID == s.serverID {
			continue
		}
		s.deliverBusMessage(msg)
	}
}

func (s *Server) deliverBusMessage(msg BusMessage) {
	switch msg.Type {
	case "public":
		s.fanoutTCP(msg.Content, msg.From)
		outgoing := wsOutgoing{
			Type: "message", Mode: "public",
			From: msg.From, Avatar: msg.Avatar,
			Content: msg.Content, Time: msg.Time,
		}
		s.wsLock.RLock()
		for _, client := range s.WSConns {
			s.writeWS(client, outgoing)
		}
		s.wsLock.RUnlock()

	case "private":
		s.wsLock.RLock()
		client, ok := s.WSConns[msg.To]
		s.wsLock.RUnlock()
		if ok {
			s.writeWS(client, wsOutgoing{
				Type: "message", Mode: "private",
				From: msg.From, To: msg.To,
				Avatar: msg.Avatar, Content: msg.Content, Time: msg.Time,
			})
		}
		s.mapLock.RLock()
		user, ok := s.OnlineMap[msg.To]
		s.mapLock.RUnlock()
		if ok {
			user.SendMsg("[priv] " + msg.From + ": " + msg.Content + "\n")
		}

	case "group":
		outgoing := wsOutgoing{
			Type: "message", Mode: "group",
			Group: msg.To, From: msg.From,
			Avatar: msg.Avatar, Content: msg.Content, Time: msg.Time,
		}
		textMsg := fmt.Sprintf("[group %s] %s: %s\n", msg.To, msg.From, msg.Content)
		s.wsLock.RLock()
		for _, client := range s.WSConns {
			s.writeWS(client, outgoing)
		}
		s.wsLock.RUnlock()
		s.mapLock.RLock()
		for _, user := range s.OnlineMap {
			user.SendMsg(textMsg)
		}
		s.mapLock.RUnlock()

	case "system":
		outgoing := wsOutgoing{Type: "system", Content: msg.Content}
		s.wsLock.RLock()
		for _, client := range s.WSConns {
			s.writeWS(client, outgoing)
		}
		s.wsLock.RUnlock()
	}
}

func sanitizeInput(input string) string {
	return sanitizeInputWithLimit(input, 500)
}

func sanitizeInputWithLimit(input string, maxLen int) string {
	for {
		stripped := htmlTagRe.ReplaceAllString(input, "")
		if stripped == input {
			break
		}
		input = stripped
	}
	if len(input) > maxLen {
		input = input[:maxLen] + "..."
	}
	return strings.TrimSpace(input)
}

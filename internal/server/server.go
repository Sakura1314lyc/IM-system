package server

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"

	"IM-system/internal/db"
	"IM-system/internal/model"
)

type Server struct {
	Ip   string
	Port int

	OnlineMap map[string]*User
	mapLock   sync.RWMutex

	WebClients map[string]*model.WebClient
	webLock    sync.RWMutex

	SessionTokens map[string]*model.Session
	sessionLock   sync.RWMutex

	Message chan string

	DB *db.Database

	loginLimiter *rateLimiter

	TLSConfig *tls.Config
	ctx       context.Context
	cancel    context.CancelFunc

	tcpListener net.Listener
	webListener net.Listener
}

func New(ip string, port int, dbPath string, enableTLS bool) *Server {
	database, err := db.InitDatabase(dbPath)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize database: %v", err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{
		Ip:            ip,
		Port:          port,
		OnlineMap:     make(map[string]*User),
		WebClients:    make(map[string]*model.WebClient),
		SessionTokens: make(map[string]*model.Session),
		Message:       make(chan string),
		DB:            database,
		loginLimiter:  newRateLimiter(10, 1*time.Minute),
		ctx:           ctx,
		cancel:        cancel,
	}

	go server.cleanupSessions()

	if enableTLS {
		cert, err := generateSelfSignedCert()
		if err != nil {
			fmt.Printf("Warning: Failed to generate TLS certificate: %v\n", err)
		} else {
			server.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
			fmt.Println("TLS encryption enabled")
		}
	}

	return server
}

func (s *Server) Start() {
	var listener net.Listener
	var err error

	if s.TLSConfig != nil {
		listener, err = tls.Listen("tcp", fmt.Sprintf("%s:%d", s.Ip, s.Port), s.TLSConfig)
		s.tcpListener = listener
		if err != nil {
			fmt.Println("TLS net.Listen err:", err)
			return
		}
		fmt.Printf("TLS TCP server listening on %s:%d\n", s.Ip, s.Port)
	} else {
		var addr string
		var listenErr error
		listener, addr, listenErr = listenWithFallback(fmt.Sprintf("%s:%d", s.Ip, s.Port), 20)
		s.tcpListener = listener
		err = listenErr
		if err != nil {
			fmt.Println("net.Listen err:", err)
			return
		}
		if _, p, splitErr := net.SplitHostPort(addr); splitErr == nil {
			var parsedPort int
			fmt.Sscanf(p, "%d", &parsedPort)
			if parsedPort > 0 {
				s.Port = parsedPort
			}
		}
		fmt.Printf("TCP server listening on %s\n", addr)
	}

	go s.ListenMessager()
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("listener accept err:", err)
			continue
		}
		go s.Handler(conn)
	}
}

func (s *Server) Shutdown() {
	s.cancel()
	if s.webListener != nil {
		s.webListener.Close()
	}
	if s.tcpListener != nil {
		s.tcpListener.Close()
	}
}

func (s *Server) Handler(conn net.Conn) {
	user := NewUser(conn, s)
	user.Online()

	isLive := make(chan bool)

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if n == 0 || err != nil {
				user.Offline()
				return
			}

			msg := strings.TrimSpace(string(buf[:n]))
			if msg == "" {
				continue
			}

			user.DoMessage(msg)
			isLive <- true
		}
	}()

	for {
		select {
		case <-isLive:
		case <-time.After(time.Minute * 10):
			fmt.Printf("[%s] 用户 %s 超时未活动，被踢出\n", time.Now().Format("2006-01-02 15:04:05"), user.Name)
			user.SendMsg("你已被踢出")
			user.Offline()
			return
		}
	}
}

func (s *Server) ListenMessager() {
	for {
		msg := <-s.Message
		fmt.Printf("[%s] 广播消息: %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)

		s.mapLock.RLock()
		for _, cli := range s.OnlineMap {
			select {
			case cli.C <- msg:
			default:
				fmt.Printf("[%s] 用户 %s 消息队列满，已跳过\n", time.Now().Format("2006-01-02 15:04:05"), cli.Name)
			}
		}
		s.mapLock.RUnlock()

		s.sendToWebClients(msg)
	}
}

func (s *Server) BroadCast(user *User, msg string) {
	msg = sanitizeInput(msg)
	sendMsg := "[" + user.Addr + "] " + user.Name + ": " + msg
	fmt.Printf("[%s] Broadcast from %s(%s): %s\n", time.Now().Format("2006-01-02 15:04:05"), user.Name, user.Addr, msg)

	fromUser, err := s.DB.GetUserByUsername(user.Name)
	if err == nil {
		_, err = s.DB.SaveMessage(fromUser.ID, nil, nil, msg, "public")
		if err != nil {
			fmt.Printf("Warning: Failed to save public message: %v\n", err)
		}
	}

	s.Message <- sendMsg
}

func (s *Server) BroadCastFromWeb(name string, msg string, avatar string) {
	msg = sanitizeInput(msg)
	sendMsg := "[WEB] " + name + ": " + msg
	fmt.Printf("[%s] Broadcast from WEB user %s: %s\n", time.Now().Format("2006-01-02 15:04:05"), name, msg)

	if fromUser, err := s.DB.GetUserByUsername(name); err == nil {
		if _, err := s.DB.SaveMessage(fromUser.ID, nil, nil, msg, "public"); err != nil {
			fmt.Printf("Warning: Failed to save web public message: %v\n", err)
		}
	}

	s.Message <- sendMsg
}

func (s *Server) BroadCastToGroup(groupName, from, msg, avatar string) error {
	msg = sanitizeInput(msg)

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
	fmt.Printf("[%s] Group broadcast to %s from %s: %s\n", time.Now().Format("2006-01-02 15:04:05"), groupName, from, msg)

	_, err = s.DB.SaveMessage(fromUser.ID, nil, &groupID, msg, "group")
	if err != nil {
		fmt.Printf("Warning: Failed to save group message: %v\n", err)
	}

	for _, member := range members {
		s.mapLock.RLock()
		if user, ok := s.OnlineMap[member.Username]; ok {
			user.SendMsg(sendMsg + "\n")
		}
		s.mapLock.RUnlock()

		s.webLock.RLock()
		if client, ok := s.WebClients[member.Username]; ok {
			select {
			case client.C <- sendMsg:
			default:
			}
		}
		s.webLock.RUnlock()
	}
	return nil
}

func (s *Server) SendPrivate(from, to, content, avatar string) error {
	content = sanitizeInput(content)

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
		fmt.Printf("Warning: Failed to save private message: %v\n", err)
	}

	s.mapLock.RLock()
	if user, ok := s.OnlineMap[to]; ok {
		user.SendMsg("[私聊] " + from + ": " + content + "\n")
		s.mapLock.RUnlock()
		return nil
	}
	s.mapLock.RUnlock()

	s.webLock.RLock()
	if client, ok := s.WebClients[to]; ok {
		select {
		case client.C <- "[私聊] " + from + ": " + content:
		default:
			fmt.Printf("Warning: web client %s message queue full, skip private message\n", to)
		}
		s.webLock.RUnlock()
		return nil
	}
	s.webLock.RUnlock()

	return fmt.Errorf("目标用户[%s]不在线", to)
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

	s.webLock.RLock()
	for name, client := range s.WebClients {
		if !seen[name] {
			seen[name] = true
			users = append(users, map[string]string{
				"name":   name,
				"avatar": client.Avatar,
			})
		}
	}
	s.webLock.RUnlock()

	return users
}

func (s *Server) IsNameTaken(name string) bool {
	s.mapLock.RLock()
	_, inTcp := s.OnlineMap[name]
	s.mapLock.RUnlock()

	if inTcp {
		return true
	}

	s.webLock.RLock()
	_, inWeb := s.WebClients[name]
	s.webLock.RUnlock()

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

func (s *Server) RenameWebClient(oldName, newName string) error {
	if s.IsNameTaken(newName) {
		return fmt.Errorf("用户名已被占用")
	}

	s.webLock.Lock()
	defer s.webLock.Unlock()

	client, ok := s.WebClients[oldName]
	if !ok {
		return fmt.Errorf("旧用户名不存在")
	}

	delete(s.WebClients, oldName)
	client.Name = newName
	s.WebClients[newName] = client
	return nil
}

func (s *Server) AddWebClient(client *model.WebClient) {
	s.webLock.Lock()
	defer s.webLock.Unlock()
	s.WebClients[client.Name] = client
}

func (s *Server) RemoveWebClient(name string) {
	s.webLock.Lock()
	defer s.webLock.Unlock()
	if c, ok := s.WebClients[name]; ok {
		close(c.C)
	}
	delete(s.WebClients, name)
}

func (s *Server) sendToWebClients(message string) {
	s.webLock.RLock()
	defer s.webLock.RUnlock()
	for _, client := range s.WebClients {
		select {
		case client.C <- message:
		default:
		}
	}
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

func (s *Server) UpdateAvatar(username, avatar string) error {
	return s.DB.UpdateUserAvatar(username, avatar)
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

func (s *Server) generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Server) CreateSession(username string) (string, error) {
	token, err := s.generateToken()
	if err != nil {
		return "", err
	}
	now := time.Now()
	s.sessionLock.Lock()
	defer s.sessionLock.Unlock()
	s.SessionTokens[token] = &model.Session{
		Username:  username,
		CreatedAt: now,
		ExpiresAt: now.Add(model.SessionTTL),
	}
	return token, nil
}

func (s *Server) GetUsernameByToken(token string) (string, bool) {
	s.sessionLock.RLock()
	sess, ok := s.SessionTokens[token]
	s.sessionLock.RUnlock()

	if !ok {
		return "", false
	}
	if sess.IsExpired() {
		s.DeleteSession(token)
		return "", false
	}
	return sess.Username, true
}

func (s *Server) UpdateSessionUsername(token, username string) {
	s.sessionLock.Lock()
	defer s.sessionLock.Unlock()
	if sess, ok := s.SessionTokens[token]; ok {
		sess.Username = username
	}
}

func (s *Server) DeleteSession(token string) {
	s.sessionLock.Lock()
	defer s.sessionLock.Unlock()
	delete(s.SessionTokens, token)
}

func (s *Server) cleanupSessions() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.sessionLock.Lock()
			for token, sess := range s.SessionTokens {
				if sess.IsExpired() {
					delete(s.SessionTokens, token)
				}
			}
			s.sessionLock.Unlock()
		case <-s.ctx.Done():
			return
		}
	}
}

func sanitizeInput(input string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	input = re.ReplaceAllString(input, "")
	if len(input) > 500 {
		input = input[:500] + "..."
	}
	return strings.TrimSpace(input)
}

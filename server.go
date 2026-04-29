package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type WebClient struct {
	Name   string
	C      chan string
	Avatar string // 用户头像
}

type ctxKey string

const ctxUser ctxKey = "user"

type Server struct {
	Ip   string
	Port int

	//在线用户的列表 (TCP 客户端)
	OnlineMap map[string]*User
	mapLock   sync.RWMutex

	//Web 前端连接列表
	WebClients map[string]*WebClient
	webLock    sync.RWMutex

	// Web 登录会话 token
	SessionTokens map[string]string
	sessionLock   sync.RWMutex

	//消息广播的channel
	Message chan string

	// 数据库连接
	DB *Database

	// TLS配置
	TLSConfig *tls.Config
	ctx       context.Context
	cancel    context.CancelFunc

	tcpListener net.Listener
	webListener net.Listener
}

// 创建 server 实例
func NewServer(ip string, port int, dbPath string, enableTLS bool) *Server {
	db, err := InitDatabase(dbPath)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize database: %v", err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{
		Ip:            ip,
		Port:          port,
		OnlineMap:     make(map[string]*User),
		WebClients:    make(map[string]*WebClient),
		SessionTokens: make(map[string]string),
		Message:       make(chan string),
		DB:            db,
		ctx:           ctx,
		cancel:        cancel,
	}

	// 如果启用TLS，生成自签名证书
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

// authQueryMiddleware extracts token from query params and sets username in request context.
func (s *Server) authQueryMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" {
			http.Error(w, "缺少登录 token", http.StatusBadRequest)
			return
		}
		username, ok := s.GetUsernameByToken(token)
		if !ok {
			http.Error(w, "认证失败", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUser, username)
		next(w, r.WithContext(ctx))
	}
}

// getUserFromContext retrieves the authenticated username from request context.
func getUserFromContext(r *http.Request) string {
	if v := r.Context().Value(ctxUser); v != nil {
		return v.(string)
	}
	return ""
}

// 验证用户凭据 - 返回(是否验证成功, 用户头像)
func (s *Server) Authenticate(username, password string) (string, bool) {
	user, err := s.DB.AuthenticateUser(username, password)
	if err != nil {
		return "", false
	}
	return user.Avatar, true
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
	s.sessionLock.Lock()
	defer s.sessionLock.Unlock()
	s.SessionTokens[token] = username
	return token, nil
}

func (s *Server) GetUsernameByToken(token string) (string, bool) {
	s.sessionLock.RLock()
	defer s.sessionLock.RUnlock()
	username, ok := s.SessionTokens[token]
	return username, ok
}

func (s *Server) UpdateSessionUsername(token, username string) {
	s.sessionLock.Lock()
	defer s.sessionLock.Unlock()
	if _, ok := s.SessionTokens[token]; ok {
		s.SessionTokens[token] = username
	}
}

func (s *Server) DeleteSession(token string) {
	s.sessionLock.Lock()
	defer s.sessionLock.Unlock()
	delete(s.SessionTokens, token)
}

// 原子性重命名用户 - 修复并发问题
func (s *Server) RenameUser(oldName, newName string) bool {
	s.mapLock.Lock()
	defer s.mapLock.Unlock()

	// 检查新名称是否已被使用
	if _, exists := s.OnlineMap[newName]; exists {
		return false
	}

	// 获取旧用户对象
	user, ok := s.OnlineMap[oldName]
	if !ok {
		return false
	}

	// 原子性地删除旧的和添加新的
	delete(s.OnlineMap, oldName)
	s.OnlineMap[newName] = user

	return true
}

// 注册新用户
func (s *Server) Register(username, password, avatar string) error {
	return s.DB.RegisterUser(username, password, avatar)
}

func (s *Server) UpdateAvatar(username, avatar string) error {
	return s.DB.UpdateUserAvatar(username, avatar)
}

// 群聊广播
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

	// 获取群组ID
	var groupID int
	err = s.DB.db.QueryRow("SELECT id FROM groups WHERE name = ?", groupName).Scan(&groupID)
	if err != nil {
		return fmt.Errorf("获取群组ID失败: %v", err)
	}

	sendMsg := fmt.Sprintf("[群聊 %s] %s: %s", groupName, from, msg)
	fmt.Printf("[%s] Group broadcast to %s from %s: %s\n", time.Now().Format("2006-01-02 15:04:05"), groupName, from, msg)

	// 保存消息到数据库
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

// 输入验证：过滤恶意内容
func sanitizeInput(input string) string {
	// 移除HTML标签
	re := regexp.MustCompile(`<[^>]*>`)
	input = re.ReplaceAllString(input, "")
	// 限制长度
	if len(input) > 500 {
		input = input[:500] + "..."
	}
	return strings.TrimSpace(input)
}

// 创建群组
func (s *Server) CreateGroup(groupName, creator string) error {
	creatorUser, err := s.DB.GetUserByUsername(creator)
	if err != nil {
		return fmt.Errorf("用户不存在: %v", err)
	}

	_, err = s.DB.CreateGroup(groupName, creatorUser.ID, "")
	return err
}

// 加入群组
func (s *Server) JoinGroup(groupName, user string) error {
	userObj, err := s.DB.GetUserByUsername(user)
	if err != nil {
		return fmt.Errorf("用户不存在: %v", err)
	}

	return s.DB.JoinGroup(groupName, userObj.ID)
}

// 离开群组
func (s *Server) LeaveGroup(groupName, user string) error {
	userObj, err := s.DB.GetUserByUsername(user)
	if err != nil {
		return fmt.Errorf("用户不存在: %v", err)
	}

	return s.DB.LeaveGroup(groupName, userObj.ID)
}

// 广播消息的方法
func (s *Server) BroadCast(user *User, msg string) {
	msg = sanitizeInput(msg)
	sendMsg := "[" + user.Addr + "] " + user.Name + ": " + msg
	fmt.Printf("[%s] Broadcast from %s(%s): %s\n", time.Now().Format("2006-01-02 15:04:05"), user.Name, user.Addr, msg)

	// 保存消息到数据库
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

func (s *Server) AddWebClient(client *WebClient) {
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

	// 保存消息到数据库
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

func (s *Server) sendToWebClients(message string) {
	s.webLock.RLock()
	defer s.webLock.RUnlock()
	for _, client := range s.WebClients {
		select {
		case client.C <- message:
		default:
			// 防止缓冲队列阻塞
		}
	}
}

func (s *Server) ListenMessager() {
	for {
		msg := <-s.Message
		fmt.Printf("[%s] 广播消息: %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)

		//将msg发送给全部在线User
		s.mapLock.RLock()
		for _, cli := range s.OnlineMap {
			select {
			case cli.C <- msg:
			default:
				// 避免阻塞，若用户管道满则略过
				fmt.Printf("[%s] 用户 %s 消息队列满，已跳过\n", time.Now().Format("2006-01-02 15:04:05"), cli.Name)
			}
		}
		s.mapLock.RUnlock()

		//发送给Web客户端
		s.sendToWebClients(msg)
	}
}

func (s *Server) Handler(conn net.Conn) {
	//.当前链接的业务
	//fmt.Println("链接建立成功")

	user := NewUser(conn, s) // 创建用户

	user.Online() // 用户上线

	//监听是否活跃的channel
	isLive := make(chan bool)

	//接收客户端发送的消息
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if n == 0 || err == io.EOF {
				user.Offline() //用户下线
				return
			}

			if err != nil {
				fmt.Println("Conn Read err:", err)
				user.Offline()
				return
			}

			// 提取用户的消息并去除首尾空白
			msg := strings.TrimSpace(string(buf[:n]))
			if msg == "" {
				continue
			}

			// 将得到的数据进行处理
			user.DoMessage(msg)

			//用户的任意消息，代表当前用户是一个活跃的
			isLive <- true
		}
	}()

	//当前handler阻塞
	for {
		select {
		case <-isLive:
			//说明当前用户时活跃的，应该重置定时器
			//不做任何事情，为了激活select，更新定时器
		case <-time.After(time.Minute * 10):
			//说明超时
			//将当前的User强制关闭

			fmt.Printf("[%s] 用户 %s 超时未活动，被踢出\n", time.Now().Format("2006-01-02 15:04:05"), user.Name)
			user.SendMsg("你已被踢出")
			user.Offline()
			return
			// 也可以用 runtime.Goexit(), 但 return 更直观

		}
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Username) == "" || strings.TrimSpace(data.Password) == "" {
		http.Error(w, "用户名和密码不能为空", http.StatusBadRequest)
		return
	}

	userInfo, err := s.DB.AuthenticateUser(data.Username, data.Password)
	if err != nil {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	token, err := s.CreateSession(data.Username)
	if err != nil {
		http.Error(w, "生成会话失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":     token,
		"avatar":    userInfo.Avatar,
		"gender":    userInfo.Gender,
		"signature": userInfo.Signature,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Token) == "" {
		http.Error(w, "token 不能为空", http.StatusBadRequest)
		return
	}

	s.DeleteSession(data.Token)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if strings.TrimSpace(token) == "" {
		http.Error(w, "缺少登录 token", http.StatusBadRequest)
		return
	}

	name, ok := s.GetUsernameByToken(token)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	userInfo, err := s.DB.GetUserByUsername(name)
	if err != nil {
		http.Error(w, "用户信息读取失败", http.StatusUnauthorized)
		return
	}

	s.webLock.RLock()
	_, exists := s.WebClients[name]
	s.webLock.RUnlock()
	if exists {
		s.RemoveWebClient(name)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	client := &WebClient{Name: name, C: make(chan string, 100), Avatar: userInfo.Avatar}
	s.AddWebClient(client)
	defer s.RemoveWebClient(name)

	fmt.Fprintf(w, "event: system\ndata: %s\n\n", "已连接到服务器")
	flusher.Flush()

	notify := r.Context().Done()

	for {
		select {
		case <-notify:
			return
		case msg, ok := <-client.C:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

func (s *Server) handleOnline(w http.ResponseWriter, r *http.Request) {
	users := s.GetOnlineUsers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"online": users})
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Token   string `json:"token"`
		Name    string `json:"name"`
		To      string `json:"to"`
		Message string `json:"message"`
		Mode    string `json:"mode"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Token) == "" || strings.TrimSpace(data.Message) == "" {
		http.Error(w, "token 和 message 不能为空", http.StatusBadRequest)
		return
	}

	name, ok := s.GetUsernameByToken(data.Token)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	if data.Mode == "private" && strings.TrimSpace(data.To) == "" {
		http.Error(w, "私聊必须指定 to", http.StatusBadRequest)
		return
	}

	if data.Mode == "group" && strings.TrimSpace(data.To) == "" {
		http.Error(w, "群聊必须指定 to (群名)", http.StatusBadRequest)
		return
	}

	avatar := ""
	s.webLock.RLock()
	if client, ok := s.WebClients[name]; ok {
		avatar = client.Avatar
	}
	s.webLock.RUnlock()

	if data.Mode == "private" {
		if err := s.SendPrivate(name, data.To, data.Message, avatar); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if data.Mode == "group" {
		if err := s.BroadCastToGroup(data.To, name, data.Message, avatar); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	s.BroadCastFromWeb(name, data.Message, avatar)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "只支持 GET 或 POST", http.StatusMethodNotAllowed)
		return
	}

	switch r.Method {
	case http.MethodGet:
		username := getUserFromContext(r)
		targetUsername := strings.TrimSpace(r.URL.Query().Get("user"))
		if targetUsername == "" {
			targetUsername = username
		}

		userInfo, err := s.DB.GetUserByUsername(targetUsername)
		if err != nil {
			http.Error(w, "用户信息读取失败", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"username":  userInfo.Username,
			"avatar":    userInfo.Avatar,
			"gender":    userInfo.Gender,
			"signature": userInfo.Signature,
			"createdAt": userInfo.CreatedAt,
			"isSelf":    userInfo.Username == username,
		})
		return

	case http.MethodPost:
		var data struct {
			Token     string `json:"token"`
			Gender    string `json:"gender"`
			Signature string `json:"signature"`
		}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, "请求体解析失败", http.StatusBadRequest)
			return
		}

		if strings.TrimSpace(data.Token) == "" {
			http.Error(w, "token 不能为空", http.StatusBadRequest)
			return
		}

		username, ok := s.GetUsernameByToken(data.Token)
		if !ok {
			http.Error(w, "认证失败", http.StatusUnauthorized)
			return
		}

		if err := s.DB.UpdateUserProfile(username, data.Gender, data.Signature); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleAvatar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Token  string `json:"token"`
		Avatar string `json:"avatar"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Token) == "" || strings.TrimSpace(data.Avatar) == "" {
		http.Error(w, "token 和 avatar 不能为空", http.StatusBadRequest)
		return
	}

	username, ok := s.GetUsernameByToken(data.Token)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	if err := s.UpdateAvatar(username, data.Avatar); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "只支持 GET", http.StatusMethodNotAllowed)
		return
	}

	username := getUserFromContext(r)

	typeResp := r.URL.Query().Get("type")
	groupName := r.URL.Query().Get("group")
	limit := 300
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		if parsedLimit, parseErr := strconv.Atoi(rawLimit); parseErr == nil && parsedLimit > 0 {
			limit = parsedLimit
			if limit > 500 {
				limit = 500
			}
		}
	}

	var history interface{}
	var err error

	switch typeResp {
	case "group":
		if strings.TrimSpace(groupName) == "" {
			http.Error(w, "缺少 group 参数", http.StatusBadRequest)
			return
		}
		history, err = s.DB.GetGroupMessages(groupName, limit)
	case "private":
		peer := r.URL.Query().Get("peer")
		if strings.TrimSpace(peer) == "" {
			http.Error(w, "缺少 peer 参数", http.StatusBadRequest)
			return
		}
		history, err = s.DB.GetPrivateMessages(username, peer, limit)
	default:
		history, err = s.DB.GetPublicMessages(limit)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"history": history})
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Token string `json:"token"`
		New   string `json:"new"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Token) == "" || strings.TrimSpace(data.New) == "" {
		http.Error(w, "token/new 不能为空", http.StatusBadRequest)
		return
	}

	username, ok := s.GetUsernameByToken(data.Token)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	if s.IsNameTaken(data.New) {
		http.Error(w, "用户名已被占用", http.StatusBadRequest)
		return
	}

	if err := s.RenameWebClient(username, data.New); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.UpdateSessionUsername(data.Token, data.New)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Avatar   string `json:"avatar"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Username) == "" || strings.TrimSpace(data.Password) == "" {
		http.Error(w, "用户名和密码不能为空", http.StatusBadRequest)
		return
	}

	if err := s.Register(data.Username, data.Password, data.Avatar); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Token      string `json:"token"`
		Action     string `json:"action"`
		GroupName  string `json:"groupName"`
		MemberName string `json:"memberName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Token) == "" || strings.TrimSpace(data.Action) == "" || strings.TrimSpace(data.GroupName) == "" {
		http.Error(w, "token、action 和 groupName 不能为空", http.StatusBadRequest)
		return
	}

	username, ok := s.GetUsernameByToken(data.Token)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	user, err := s.DB.GetUserByUsername(username)
	if err != nil {
		http.Error(w, "用户信息读取失败", http.StatusInternalServerError)
		return
	}

	switch data.Action {
	case "create":
		if _, err := s.DB.CreateGroup(data.GroupName, user.ID, ""); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	case "join":
		if err := s.DB.JoinGroup(data.GroupName, user.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	case "leave":
		if err := s.DB.LeaveGroup(data.GroupName, user.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	case "invite":
		if strings.TrimSpace(data.MemberName) == "" {
			http.Error(w, "memberName 不能为空", http.StatusBadRequest)
			return
		}
		targetUser, err := s.DB.GetUserByUsername(data.MemberName)
		if err != nil {
			http.Error(w, "目标用户不存在", http.StatusNotFound)
			return
		}
		if err := s.DB.JoinGroup(data.GroupName, targetUser.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	case "kick":
		if strings.TrimSpace(data.MemberName) == "" {
			http.Error(w, "memberName 不能为空", http.StatusBadRequest)
			return
		}
		group, err := s.DB.GetGroupByName(data.GroupName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if group.CreatorID != user.ID {
			http.Error(w, "只有群主可以踢人", http.StatusForbidden)
			return
		}
		targetUser, err := s.DB.GetUserByUsername(data.MemberName)
		if err != nil {
			http.Error(w, "目标用户不存在", http.StatusNotFound)
			return
		}
		if err := s.DB.LeaveGroup(data.GroupName, targetUser.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "未知 action", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "只支持 GET", http.StatusMethodNotAllowed)
		return
	}

	username := getUserFromContext(r)

	groups, err := s.DB.GetUserGroups(username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"groups": groups})
}

// 处理好友操作（加好友/删好友）
func (s *Server) handleFriend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Token      string `json:"token"`
		Action     string `json:"action"`
		FriendName string `json:"friend"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "无效的请求格式", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Token) == "" {
		http.Error(w, "缺少登录 token", http.StatusBadRequest)
		return
	}

	username, ok := s.GetUsernameByToken(data.Token)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	user, err := s.DB.GetUserByUsername(username)
	if err != nil {
		http.Error(w, "用户不存在", http.StatusUnauthorized)
		return
	}

	if strings.TrimSpace(data.FriendName) == "" {
		http.Error(w, "friend 不能为空", http.StatusBadRequest)
		return
	}

	friendUser, err := s.DB.GetUserByUsername(data.FriendName)
	if err != nil {
		http.Error(w, "好友用户不存在", http.StatusNotFound)
		return
	}

	switch data.Action {
	case "add":
		if err := s.DB.AddFriend(user.ID, friendUser.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	case "remove":
		if err := s.DB.RemoveFriend(user.ID, friendUser.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "未知 action", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// 获取好友列表
func (s *Server) handleFriends(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "只支持 GET", http.StatusMethodNotAllowed)
		return
	}

	username := getUserFromContext(r)

	user, err := s.DB.GetUserByUsername(username)
	if err != nil {
		http.Error(w, "用户不存在", http.StatusUnauthorized)
		return
	}

	friends, err := s.DB.GetFriends(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回好友的用户名和头像
	var friendList []map[string]interface{}
	for _, f := range friends {
		friendList = append(friendList, map[string]interface{}{
			"name":   f.Username,
			"avatar": f.Avatar,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"friends": friendList})
}

// 检查是否是好友
func (s *Server) handleCheckFriend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "只支持 GET", http.StatusMethodNotAllowed)
		return
	}

	friendName := r.URL.Query().Get("friend")
	if strings.TrimSpace(friendName) == "" {
		http.Error(w, "缺少必要参数", http.StatusBadRequest)
		return
	}

	username := getUserFromContext(r)

	user, err := s.DB.GetUserByUsername(username)
	if err != nil {
		http.Error(w, "用户不存在", http.StatusUnauthorized)
		return
	}

	friendUser, err := s.DB.GetUserByUsername(friendName)
	if err != nil {
		http.Error(w, "好友用户不存在", http.StatusNotFound)
		return
	}

	isFriend, err := s.DB.IsFriend(user.ID, friendUser.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"isFriend": isFriend})
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown() {
	s.cancel()
	if s.webListener != nil {
		s.webListener.Close()
	}
	if s.tcpListener != nil {
		s.tcpListener.Close()
	}
}

func (s *Server) StartWeb(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/", noCache(http.FileServer(http.Dir("./web"))))
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/online", s.handleOnline)
	mux.HandleFunc("/api/send", s.handleSend)
	mux.HandleFunc("/api/rename", s.handleRename)
	mux.HandleFunc("/api/register", s.handleRegister)
	mux.HandleFunc("/api/avatar", s.handleAvatar)
	mux.HandleFunc("/api/group", s.handleGroup)
	mux.HandleFunc("/api/groups", s.authQueryMiddleware(s.handleGroups))
	mux.HandleFunc("/api/profile", s.handleProfile)
	mux.HandleFunc("/api/history", s.authQueryMiddleware(s.handleHistory))
	mux.HandleFunc("/api/friend", s.handleFriend)
	mux.HandleFunc("/api/friends", s.authQueryMiddleware(s.handleFriends))
	mux.HandleFunc("/api/check-friend", s.authQueryMiddleware(s.handleCheckFriend))

	listener, finalAddr, err := listenWithFallback(addr, 20)
	if err != nil {
		fmt.Println("StartWeb err:", err)
		return
	}
	s.webListener = listener
	defer func() { s.webListener = nil }()

	fmt.Printf("[%s] Web UI 服务启动 %s\n", time.Now().Format("2006-01-02 15:04:05"), finalAddr)
	if err := http.Serve(listener, mux); err != nil {
		fmt.Println("StartWeb serve err:", err)
	}
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

// 启动服务器的接口
func (s *Server) Start() {
	//socket listen
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

	//close listen socket
	s.webListener = listener
	defer func() { s.webListener = nil }()

	//启动监听msg的goroutine
	go s.ListenMessager()
	for {
		//accept
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("listener accept err:", err)
			continue
		}

		//do handler
		go s.Handler(conn)

	}
}

func listenWithFallback(addr string, maxAttempts int) (net.Listener, string, error) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, "", err
	}

	var port int
	fmt.Sscanf(portStr, "%d", &port)
	if port <= 0 {
		return nil, "", fmt.Errorf("invalid port in address: %s", addr)
	}

	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		tryAddr := net.JoinHostPort(host, fmt.Sprintf("%d", port+i))
		ln, listenErr := net.Listen("tcp", tryAddr)
		if listenErr == nil {
			if i > 0 {
				fmt.Printf("端口 %s 被占用，已自动切换到 %s\n", addr, tryAddr)
			}
			return ln, tryAddr, nil
		}
		lastErr = listenErr
		if !isAddrInUseErr(listenErr) {
			return nil, "", listenErr
		}
	}

	return nil, "", fmt.Errorf("listen failed after %d attempts: %w", maxAttempts, lastErr)
}

func isAddrInUseErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "address already in use") ||
		strings.Contains(msg, "only one usage")
}

// generateSelfSignedCert creates an ephemeral self-signed TLS certificate for development.
func generateSelfSignedCert() (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "localhost",
			Organization: []string{"IM-system Dev"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", "127.0.0.1"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create certificate: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}, nil
}

package main

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

type WebClient struct {
	Name   string
	C      chan string
	Avatar string // 用户头像
}

// 使用更好的类型定义（兼容旧版本）
type UserCred struct {
	Password string
	Avatar   string // 添加头像支持
}

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
}

// 创建 server 实例
func NewServer(ip string, port int, dbPath string, enableTLS bool) *Server {
	db, err := InitDatabase(dbPath)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize database: %v", err))
	}

	server := &Server{
		Ip:            ip,
		Port:          port,
		OnlineMap:     make(map[string]*User),
		WebClients:    make(map[string]*WebClient),
		SessionTokens: make(map[string]string),
		Message:       make(chan string),
		DB:            db,
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

// 验证用户凭据 - 返回(是否验证成功, 用户头像)
func (this *Server) Authenticate(username, password string) (string, bool) {
	user, err := this.DB.AuthenticateUser(username, password)
	if err != nil {
		return "", false
	}
	return user.Avatar, true
}

func (this *Server) generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (this *Server) CreateSession(username string) (string, error) {
	token, err := this.generateToken()
	if err != nil {
		return "", err
	}
	this.sessionLock.Lock()
	defer this.sessionLock.Unlock()
	this.SessionTokens[token] = username
	return token, nil
}

func (this *Server) GetUsernameByToken(token string) (string, bool) {
	this.sessionLock.RLock()
	defer this.sessionLock.RUnlock()
	username, ok := this.SessionTokens[token]
	return username, ok
}

func (this *Server) UpdateSessionUsername(token, username string) {
	this.sessionLock.Lock()
	defer this.sessionLock.Unlock()
	if _, ok := this.SessionTokens[token]; ok {
		this.SessionTokens[token] = username
	}
}

func (this *Server) DeleteSession(token string) {
	this.sessionLock.Lock()
	defer this.sessionLock.Unlock()
	delete(this.SessionTokens, token)
}

// 原子性重命名用户 - 修复并发问题
func (this *Server) RenameUser(oldName, newName string) bool {
	this.mapLock.Lock()
	defer this.mapLock.Unlock()

	// 检查新名称是否已被使用
	if _, exists := this.OnlineMap[newName]; exists {
		return false
	}

	// 获取旧用户对象
	user, ok := this.OnlineMap[oldName]
	if !ok {
		return false
	}

	// 原子性地删除旧的和添加新的
	delete(this.OnlineMap, oldName)
	this.OnlineMap[newName] = user

	return true
}

// 注册新用户
func (this *Server) Register(username, password, avatar string) error {
	if avatar == "" {
		avatar = "👤"
	}
	return this.DB.RegisterUser(username, password, avatar)
}

func (this *Server) UpdateAvatar(username, avatar string) error {
	return this.DB.UpdateUserAvatar(username, avatar)
}

// 群聊广播
func (this *Server) BroadCastToGroup(groupName, from, msg, avatar string) error {
	msg = sanitizeInput(msg)

	fromUser, err := this.DB.GetUserByUsername(from)
	if err != nil {
		return fmt.Errorf("发送者不存在: %v", err)
	}

	members, err := this.DB.GetGroupMembers(groupName)
	if err != nil {
		return fmt.Errorf("获取群组成员失败: %v", err)
	}

	if len(members) == 0 {
		return fmt.Errorf("群组不存在或没有成员")
	}

	// 获取群组ID
	var groupID int
	err = this.DB.db.QueryRow("SELECT id FROM groups WHERE name = ?", groupName).Scan(&groupID)
	if err != nil {
		return fmt.Errorf("获取群组ID失败: %v", err)
	}

	sendMsg := fmt.Sprintf("[群聊 %s] %s %s: %s", groupName, avatar, from, msg)
	fmt.Printf("[%s] Group broadcast to %s from %s: %s\n", time.Now().Format("2006-01-02 15:04:05"), groupName, from, msg)

	// 保存消息到数据库
	_, err = this.DB.SaveMessage(fromUser.ID, nil, &groupID, msg, "group")
	if err != nil {
		fmt.Printf("Warning: Failed to save group message: %v\n", err)
	}

	for _, member := range members {
		this.mapLock.RLock()
		if user, ok := this.OnlineMap[member.Username]; ok {
			user.SendMsg(sendMsg + "\n")
		}
		this.mapLock.RUnlock()

		this.webLock.RLock()
		if client, ok := this.WebClients[member.Username]; ok {
			select {
			case client.C <- sendMsg:
			default:
			}
		}
		this.webLock.RUnlock()
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
func (this *Server) CreateGroup(groupName, creator string) error {
	creatorUser, err := this.DB.GetUserByUsername(creator)
	if err != nil {
		return fmt.Errorf("用户不存在: %v", err)
	}

	_, err = this.DB.CreateGroup(groupName, creatorUser.ID, "")
	return err
}

// 加入群组
func (this *Server) JoinGroup(groupName, user string) error {
	userObj, err := this.DB.GetUserByUsername(user)
	if err != nil {
		return fmt.Errorf("用户不存在: %v", err)
	}

	return this.DB.JoinGroup(groupName, userObj.ID)
}

// 离开群组
func (this *Server) LeaveGroup(groupName, user string) error {
	userObj, err := this.DB.GetUserByUsername(user)
	if err != nil {
		return fmt.Errorf("用户不存在: %v", err)
	}

	return this.DB.LeaveGroup(groupName, userObj.ID)
}

// 广播消息的方法
func (this *Server) BroadCast(user *User, msg string) {
	msg = sanitizeInput(msg)
	sendMsg := "[" + user.Addr + "] " + user.Avatar + " " + user.Name + ": " + msg
	fmt.Printf("[%s] Broadcast from %s(%s): %s\n", time.Now().Format("2006-01-02 15:04:05"), user.Name, user.Addr, msg)

	// 保存消息到数据库
	fromUser, err := this.DB.GetUserByUsername(user.Name)
	if err == nil {
		_, err = this.DB.SaveMessage(fromUser.ID, nil, nil, msg, "public")
		if err != nil {
			fmt.Printf("Warning: Failed to save public message: %v\n", err)
		}
	}

	this.Message <- sendMsg
}

func (this *Server) BroadCastFromWeb(name string, msg string, avatar string) {
	msg = sanitizeInput(msg)
	sendMsg := "[WEB] " + avatar + " " + name + ": " + msg
	fmt.Printf("[%s] Broadcast from WEB user %s: %s\n", time.Now().Format("2006-01-02 15:04:05"), name, msg)
	this.Message <- sendMsg
}

func (this *Server) AddWebClient(client *WebClient) {
	this.webLock.Lock()
	defer this.webLock.Unlock()
	this.WebClients[client.Name] = client
}

func (this *Server) RemoveWebClient(name string) {
	this.webLock.Lock()
	defer this.webLock.Unlock()
	if c, ok := this.WebClients[name]; ok {
		close(c.C)
	}
	delete(this.WebClients, name)
}

func (this *Server) GetOnlineUsers() []map[string]string {
	seen := make(map[string]bool)
	users := make([]map[string]string, 0)

	this.mapLock.RLock()
	for _, u := range this.OnlineMap {
		if !seen[u.Name] {
			seen[u.Name] = true
			users = append(users, map[string]string{
				"name":   u.Name,
				"avatar": u.Avatar,
			})
		}
	}
	this.mapLock.RUnlock()

	this.webLock.RLock()
	for name, client := range this.WebClients {
		if !seen[name] {
			seen[name] = true
			users = append(users, map[string]string{
				"name":   name,
				"avatar": client.Avatar,
			})
		}
	}
	this.webLock.RUnlock()

	return users
}

func (this *Server) IsNameTaken(name string) bool {
	this.mapLock.RLock()
	_, inTcp := this.OnlineMap[name]
	this.mapLock.RUnlock()

	if inTcp {
		return true
	}

	this.webLock.RLock()
	_, inWeb := this.WebClients[name]
	this.webLock.RUnlock()

	return inWeb
}

func (this *Server) RenameWebClient(oldName, newName string) error {
	if this.IsNameTaken(newName) {
		return fmt.Errorf("用户名已被占用")
	}

	this.webLock.Lock()
	defer this.webLock.Unlock()

	client, ok := this.WebClients[oldName]
	if !ok {
		return fmt.Errorf("旧用户名不存在")
	}

	delete(this.WebClients, oldName)
	client.Name = newName
	this.WebClients[newName] = client
	return nil
}

func (this *Server) SendPrivate(from, to, content, avatar string) error {
	content = sanitizeInput(content)

	fromUser, err := this.DB.GetUserByUsername(from)
	if err != nil {
		return fmt.Errorf("发送者不存在: %v", err)
	}

	toUser, err := this.DB.GetUserByUsername(to)
	if err != nil {
		return fmt.Errorf("接收者不存在: %v", err)
	}

	// 保存消息到数据库
	_, err = this.DB.SaveMessage(fromUser.ID, &toUser.ID, nil, content, "private")
	if err != nil {
		fmt.Printf("Warning: Failed to save private message: %v\n", err)
	}

	this.mapLock.RLock()
	if user, ok := this.OnlineMap[to]; ok {
		user.SendMsg("[私聊] " + avatar + " " + from + ": " + content + "\n")
		this.mapLock.RUnlock()
		return nil
	}
	this.mapLock.RUnlock()

	this.webLock.RLock()
	if client, ok := this.WebClients[to]; ok {
		select {
		case client.C <- "[私聊] " + avatar + " " + from + ": " + content:
		default:
			fmt.Printf("Warning: web client %s message queue full, skip private message\n", to)
		}
		this.webLock.RUnlock()
		return nil
	}
	this.webLock.RUnlock()

	return fmt.Errorf("目标用户[%s]不在线", to)
}

func (this *Server) sendToWebClients(message string) {
	this.webLock.RLock()
	defer this.webLock.RUnlock()
	for _, client := range this.WebClients {
		select {
		case client.C <- message:
		default:
			// 防止缓冲队列阻塞
		}
	}
}

func (this *Server) ListenMessager() {
	for {
		msg := <-this.Message
		fmt.Printf("[%s] 广播消息: %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)

		//将msg发送给全部在线User
		this.mapLock.RLock()
		for _, cli := range this.OnlineMap {
			select {
			case cli.C <- msg:
			default:
				// 避免阻塞，若用户管道满则略过
				fmt.Printf("[%s] 用户 %s 消息队列满，已跳过\n", time.Now().Format("2006-01-02 15:04:05"), cli.Name)
			}
		}
		this.mapLock.RUnlock()

		//发送给Web客户端
		this.sendToWebClients(msg)
	}
}

func (this *Server) Handler(conn net.Conn) {
	//.当前链接的业务
	//fmt.Println("链接建立成功")

	user := NewUser(conn, this) // 创建用户

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

func (this *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
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

	avatar, ok := this.Authenticate(data.Username, data.Password)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	token, err := this.CreateSession(data.Username)
	if err != nil {
		http.Error(w, "生成会话失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token, "avatar": avatar})
}

func (this *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
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

	this.DeleteSession(data.Token)
	w.WriteHeader(http.StatusNoContent)
}

func (this *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if strings.TrimSpace(token) == "" {
		http.Error(w, "缺少登录 token", http.StatusBadRequest)
		return
	}

	name, ok := this.GetUsernameByToken(token)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	userInfo, err := this.DB.GetUserByUsername(name)
	if err != nil {
		http.Error(w, "用户信息读取失败", http.StatusUnauthorized)
		return
	}

	this.webLock.RLock()
	_, exists := this.WebClients[name]
	this.webLock.RUnlock()
	if exists {
		this.RemoveWebClient(name)
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
	this.AddWebClient(client)
	defer this.RemoveWebClient(name)

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

func (this *Server) handleOnline(w http.ResponseWriter, r *http.Request) {
	users := this.GetOnlineUsers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"online": users})
}

func (this *Server) handleSend(w http.ResponseWriter, r *http.Request) {
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

	name, ok := this.GetUsernameByToken(data.Token)
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
	this.webLock.RLock()
	if client, ok := this.WebClients[name]; ok {
		avatar = client.Avatar
	}
	this.webLock.RUnlock()

	if data.Mode == "private" {
		if err := this.SendPrivate(name, data.To, data.Message, avatar); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if data.Mode == "group" {
		if err := this.BroadCastToGroup(data.To, name, data.Message, avatar); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	this.BroadCastFromWeb(name, data.Message, avatar)
	w.WriteHeader(http.StatusNoContent)
}

func (this *Server) handleAvatar(w http.ResponseWriter, r *http.Request) {
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

	username, ok := this.GetUsernameByToken(data.Token)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	if err := this.UpdateAvatar(username, data.Avatar); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (this *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "只支持 GET", http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get("token")
	if strings.TrimSpace(token) == "" {
		http.Error(w, "缺少登录 token", http.StatusBadRequest)
		return
	}

	username, ok := this.GetUsernameByToken(token)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	typeResp := r.URL.Query().Get("type")
	groupName := r.URL.Query().Get("group")
	limit := 50

	var history interface{}
	var err error

	switch typeResp {
	case "group":
		if strings.TrimSpace(groupName) == "" {
			http.Error(w, "缺少 group 参数", http.StatusBadRequest)
			return
		}
		history, err = this.DB.GetGroupMessages(groupName, limit)
	case "private":
		peer := r.URL.Query().Get("peer")
		if strings.TrimSpace(peer) == "" {
			http.Error(w, "缺少 peer 参数", http.StatusBadRequest)
			return
		}
		history, err = this.DB.GetPrivateMessages(username, peer, limit)
	default:
		history, err = this.DB.GetPublicMessages(limit)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"history": history})
}

func (this *Server) handleRename(w http.ResponseWriter, r *http.Request) {
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

	username, ok := this.GetUsernameByToken(data.Token)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	if this.IsNameTaken(data.New) {
		http.Error(w, "用户名已被占用", http.StatusBadRequest)
		return
	}

	if err := this.RenameWebClient(username, data.New); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	this.UpdateSessionUsername(data.Token, data.New)
	w.WriteHeader(http.StatusNoContent)
}

func (this *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
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

	if err := this.Register(data.Username, data.Password, data.Avatar); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (this *Server) handleGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Token     string `json:"token"`
		Action    string `json:"action"`
		GroupName string `json:"groupName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Token) == "" || strings.TrimSpace(data.Action) == "" || strings.TrimSpace(data.GroupName) == "" {
		http.Error(w, "token、action 和 groupName 不能为空", http.StatusBadRequest)
		return
	}

	username, ok := this.GetUsernameByToken(data.Token)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	user, err := this.DB.GetUserByUsername(username)
	if err != nil {
		http.Error(w, "用户信息读取失败", http.StatusInternalServerError)
		return
	}

	switch data.Action {
	case "create":
		if _, err := this.DB.CreateGroup(data.GroupName, user.ID, ""); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	case "join":
		if err := this.DB.JoinGroup(data.GroupName, user.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	case "leave":
		if err := this.DB.LeaveGroup(data.GroupName, user.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "未知 action", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (this *Server) handleGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "只支持 GET", http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get("token")
	if strings.TrimSpace(token) == "" {
		http.Error(w, "缺少登录 token", http.StatusBadRequest)
		return
	}

	username, ok := this.GetUsernameByToken(token)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	groups, err := this.DB.GetUserGroups(username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"groups": groups})
}

func (this *Server) StartWeb(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("./web")))
	mux.HandleFunc("/api/login", this.handleLogin)
	mux.HandleFunc("/api/logout", this.handleLogout)
	mux.HandleFunc("/api/events", this.handleEvents)
	mux.HandleFunc("/api/online", this.handleOnline)
	mux.HandleFunc("/api/send", this.handleSend)
	mux.HandleFunc("/api/rename", this.handleRename)
	mux.HandleFunc("/api/register", this.handleRegister)
	mux.HandleFunc("/api/avatar", this.handleAvatar)
	mux.HandleFunc("/api/group", this.handleGroup)
	mux.HandleFunc("/api/groups", this.handleGroups)
	mux.HandleFunc("/api/history", this.handleHistory)

	fmt.Printf("[%s] Web UI 服务启动 %s\n", time.Now().Format("2006-01-02 15:04:05"), addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Println("StartWeb err:", err)
	}
}

// 启动服务器的接口
func (this *Server) Start() {
	//socket listen
	var listener net.Listener
	var err error

	if this.TLSConfig != nil {
		listener, err = tls.Listen("tcp", fmt.Sprintf("%s:%d", this.Ip, this.Port), this.TLSConfig)
		if err != nil {
			fmt.Println("TLS net.Listen err:", err)
			return
		}
		fmt.Printf("TLS TCP server listening on %s:%d\n", this.Ip, this.Port)
	} else {
		listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", this.Ip, this.Port))
		if err != nil {
			fmt.Println("net.Listen err:", err)
			return
		}
		fmt.Printf("TCP server listening on %s:%d\n", this.Ip, this.Port)
	}

	//close listen socket
	defer listener.Close()

	//启动监听msg的goroutine
	go this.ListenMessager()
	for {
		//accept
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("listener accept err:", err)
			continue
		}

		//do handler
		go this.Handler(conn)

	}
}

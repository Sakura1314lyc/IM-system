package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type WebClient struct {
	Name string
	C    chan string
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

	//消息广播的channel
	Message chan string
}

// 创建server接口
func Newserver(ip string, port int) *Server {
	server := &Server{
		Ip:         ip,
		Port:       port,
		OnlineMap:  make(map[string]*User),
		WebClients: make(map[string]*WebClient),
		Message:    make(chan string),
	}

	return server
}

// 广播消息的方法
func (this *Server) BroadCast(user *User, msg string) {
	sendMsg := "[" + user.Addr + "]" + user.Name + ":" + msg
	fmt.Printf("[%s] Broadcast from %s(%s): %s\n", time.Now().Format("2006-01-02 15:04:05"), user.Name, user.Addr, msg)
	this.Message <- sendMsg
}

func (this *Server) BroadCastFromWeb(name string, msg string) {
	sendMsg := "[WEB]" + name + ":" + msg
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

func (this *Server) GetOnlineUsers() []string {
	users := make([]string, 0)

	this.mapLock.RLock()
	for _, u := range this.OnlineMap {
		users = append(users, u.Name)
	}
	this.mapLock.RUnlock()

	this.webLock.RLock()
	for name := range this.WebClients {
		users = append(users, name)
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
	this.webLock.Lock()
	defer this.webLock.Unlock()

	if _, exists := this.WebClients[newName]; exists {
		return fmt.Errorf("用户名已被占用")
	}

	client, ok := this.WebClients[oldName]
	if !ok {
		return fmt.Errorf("旧用户名不存在")
	}

	delete(this.WebClients, oldName)
	client.Name = newName
	this.WebClients[newName] = client
	return nil
}

func (this *Server) SendPrivate(from, to, content string) error {
	this.mapLock.RLock()
	if user, ok := this.OnlineMap[to]; ok {
		user.SendMsg("[私聊] " + from + ": " + content + "\n")
		this.mapLock.RUnlock()
		return nil
	}
	this.mapLock.RUnlock()

	this.webLock.RLock()
	if client, ok := this.WebClients[to]; ok {
		client.C <- "[私聊] " + from + ": " + content
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

func (this *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "缺少用户名参数 name", http.StatusBadRequest)
		return
	}

	if this.IsNameTaken(name) {
		// 重名允许同名从旧连接替换
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	client := &WebClient{Name: name, C: make(chan string, 100)}
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
	json.NewEncoder(w).Encode(map[string]any{"online": users})
}

func (this *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Name    string `json:"name"`
		To      string `json:"to"`
		Message string `json:"message"`
		Mode    string `json:"mode"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Name) == "" || strings.TrimSpace(data.Message) == "" {
		http.Error(w, "name 和 message 不能为空", http.StatusBadRequest)
		return
	}

	if data.Mode == "private" && strings.TrimSpace(data.To) == "" {
		http.Error(w, "私聊必须指定 to", http.StatusBadRequest)
		return
	}

	if data.Mode == "private" {
		if err := this.SendPrivate(data.Name, data.To, data.Message); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	this.BroadCastFromWeb(data.Name, data.Message)
	w.WriteHeader(http.StatusNoContent)
}

func (this *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Old) == "" || strings.TrimSpace(data.New) == "" {
		http.Error(w, "old/new 不能为空", http.StatusBadRequest)
		return
	}

	if this.IsNameTaken(data.New) {
		http.Error(w, "用户名已被占用", http.StatusBadRequest)
		return
	}

	if err := this.RenameWebClient(data.Old, data.New); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (this *Server) StartWeb(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("./web")))
	mux.HandleFunc("/api/events", this.handleEvents)
	mux.HandleFunc("/api/online", this.handleOnline)
	mux.HandleFunc("/api/send", this.handleSend)
	mux.HandleFunc("/api/rename", this.handleRename)

	fmt.Printf("[%s] Web UI 服务启动 %s\n", time.Now().Format("2006-01-02 15:04:05"), addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Println("StartWeb err:", err)
	}
}

// 启动服务器的接口
func (this *Server) Start() {
	//socket listen
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", this.Ip, this.Port))
	if err != nil {
		fmt.Println("net.Listen err:", err)
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

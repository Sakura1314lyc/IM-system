package main

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

type Server struct {
	Ip   string
	Port int

	//在线用户的列表
	OnlineMap map[string]*User
	mapLock   sync.RWMutex

	//消息广播的channel
	Message chan string
}

// 创建server接口
func Newserver(ip string, port int) *Server {
	server := &Server{
		Ip:        ip,
		Port:      port,
		OnlineMap: make(map[string]*User),
		Message:   make(chan string),
	}

	return server
}

// 监听Message广播消息channel的goroutine,一旦有消息就发送给全部的在线User
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
	}
}

// 广播消息的方法
func (this *Server) BroadCast(user *User, msg string) {
	sendMsg := "[" + user.Addr + "]" + user.Name + ":" + msg
	fmt.Printf("[%s] Broadcast from %s(%s): %s\n", time.Now().Format("2006-01-02 15:04:05"), user.Name, user.Addr, msg)
	this.Message <- sendMsg
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

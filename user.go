package main

import (
	"fmt"
	"net"
	"strings"
)

type User struct {
	Name string
	Addr string
	C    chan string
	conn net.Conn

	Server *Server
}

// 创建用户的API
func NewUser(conn net.Conn, server *Server) *User {
	userAddr := conn.RemoteAddr().String()
	user := &User{
		Name:   userAddr,
		Addr:   userAddr,
		C:      make(chan string),
		conn:   conn,
		Server: server,
	}

	//启动监听当前user channel消息的goroutine
	go user.ListenMessage()

	return user
}

// 用户上线的业务
func (this *User) Online() {

	//用户上线 将用户加入到onlineMap中
	this.Server.mapLock.Lock()
	this.Server.OnlineMap[this.Name] = this
	this.Server.mapLock.Unlock()

	//广播当前用户上线消息
	this.Server.BroadCast(this, "已上线")
}

// 给当前user对应的客户端发送消息
func (this *User) SendMsg(msg string) {
	_, err := this.conn.Write([]byte(msg))
	if err != nil {
		fmt.Println("SendMsg err:", err)
		this.Offline()
	}
}

// 用户处理消息的业务
func (this *User) DoMessage(msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "Who" {
		// 查询当前有哪些在线用户
		this.Server.mapLock.RLock()
		for _, user := range this.Server.OnlineMap {
			onlineMsg := "[" + user.Addr + "]" + user.Name + ":" + "在线...\n"
			this.SendMsg(onlineMsg)
		}
		this.Server.mapLock.RUnlock()
		return
	}

	if strings.HasPrefix(msg, "rename|") {
		parts := strings.SplitN(msg, "|", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
			this.SendMsg("rename 命令格式错误,示例:rename|newname\n")
			return
		}

		newName := strings.TrimSpace(parts[1])

		this.Server.mapLock.RLock()
		_, exists := this.Server.OnlineMap[newName]
		this.Server.mapLock.RUnlock()
		if exists {
			this.SendMsg("当前用户名已被使用\n")
			return
		}

		this.Server.mapLock.Lock()
		delete(this.Server.OnlineMap, this.Name)
		this.Name = newName
		this.Server.OnlineMap[this.Name] = this
		this.Server.mapLock.Unlock()

		this.SendMsg("您已成功更新用户名:" + this.Name + "\n")
		this.Server.BroadCast(this, "已改名为 -> "+this.Name)
		return
	}

	this.Server.BroadCast(this, msg)
}

// 用户下线的业务
func (this *User) Offline() {
	this.Server.mapLock.Lock()
	_, ok := this.Server.OnlineMap[this.Name]
	if ok {
		delete(this.Server.OnlineMap, this.Name)
	}
	this.Server.mapLock.Unlock()

	// 广播当前用户下线消息
	this.Server.BroadCast(this, "已下线")

	if this.conn != nil {
		this.conn.Close()
	}

	select {
	case <-this.C:
	default:
	}
	close(this.C)
}

// 监听当前User channel的方法，一旦有消息，就直接发送给对端客户端
func (this *User) ListenMessage() {
	for {
		msg, ok := <-this.C
		if !ok {
			return
		}

		_, err := this.conn.Write([]byte(msg + "\n"))
		if err != nil {
			fmt.Println("ListenMessage Write err:", err)
			return
		}
	}
}

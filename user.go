package main

import "net"

type User struct {
	Name string
	Addr string
	C    chan string
	conn net.Conn

	Server *Server
}

//创建用户的API
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

//用户上线的业务
func (this *User) Online() {

	//用户上线 将用户加入到onlineMap中
	this.Server.mapLock.Lock()
	this.Server.OnlineMap[this.Name] = this
	this.Server.mapLock.Unlock()

	//广播当前用户上线消息
	this.Server.BroadCast(this, "已上线")
}

//给当前user对应的客户端发送消息
func (this *User) SendMsg(msg string) {
	this.conn.Write([]byte(msg))
}

//用户处理消息的业务
func (this *User) DoMessage(msg string) {
	if msg == "who" {
		//查询当前有哪些在线用户

		this.Server.mapLock.Lock()
		for _, user := range this.Server.OnlineMap {
			onlineMsg := "[" + user.Addr + "]" + user.Name + ":" + "在线...\n"
			this.SendMsg(onlineMsg)
		}
		this.Server.mapLock.Unlock()

	} else {
		this.Server.BroadCast(this, msg)
	}
}

//用户下线的业务
func (this *User) Offline() {

	//用户下线 将用户从onlineMap中删除
	this.Server.mapLock.Lock()
	delete(this.Server.OnlineMap, this.Name)
	this.Server.mapLock.Unlock()

	//广播当前用户下线消息
	this.Server.BroadCast(this, "已下线")
}

//监听当前User channel的方法，一旦有消息，就直接发送给对端客户端
func (this *User) ListenMessage() {
	for {
		msg := <-this.C

		this.conn.Write([]byte(msg + "\n"))
	}
}

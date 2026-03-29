package main

import (
	"fmt"
	"net"
	"strings"
	"time"
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
		C:      make(chan string, 100), // 容量防止慢客户端阻塞广播
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

	fmt.Printf("[%s] 用户 %s(%s) 上线\n", time.Now().Format("2006-01-02 15:04:05"), this.Name, this.Addr)

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
	lower := strings.ToLower(msg)
	if lower == "who" {
		// 查询当前有哪些在线用户
		this.Server.mapLock.RLock()
		for _, user := range this.Server.OnlineMap {
			onlineMsg := "[" + user.Addr + "]" + user.Name + ":" + "在线...\n"
			this.SendMsg(onlineMsg)
		}
		this.Server.mapLock.RUnlock()
		return
	}

	if strings.HasPrefix(lower, "rename|") {
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

		if strings.HasPrefix(msg, "to|") {
			//私聊

			//获取用户名
			remoteName := strings.Split(msg, "|")[1]
			if remoteName == "" {
				this.SendMsg("消息格式不正确, 示例: to|username|你好啊\n")
				return
			}

			//根据用户名 得到对方User对象
			remoteUser, ok := this.Server.OnlineMap[remoteName]
			if !ok {
				this.SendMsg("该用户名不存在\n")
				return
			}
			//获取内容 将消息发送过去

			content := strings.Split(msg, "|")[2]
			if content == "" {
				this.SendMsg("无消息内容,请重新发送\n")
				return
			}
			remoteUser.SendMsg(this.Name + "对您说" + content)
		}
		this.Server.mapLock.Lock()
		delete(this.Server.OnlineMap, this.Name)
		this.Name = newName
		this.Server.OnlineMap[this.Name] = this
		this.Server.mapLock.Unlock()

		this.SendMsg("您已成功更新用户名:" + this.Name + "\n")
		// 改名不对所有人广播，避免暴露历史操作，仅在线列表响应变化
		return
	}

	if strings.HasPrefix(lower, "to|") {
		parts := strings.SplitN(msg, "|", 3)
		if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
			this.SendMsg("私聊命令格式错误,示例:to|username|hello\n")
			return
		}

		targetName := strings.TrimSpace(parts[1])
		content := strings.TrimSpace(parts[2])

		if err := this.Server.SendPrivate(this.Name, targetName, content); err != nil {
			this.SendMsg(err.Error() + "\n")
			return
		}

		this.SendMsg("[已发送] " + content + "\n")
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

	fmt.Printf("[%s] 用户 %s 离线\n", time.Now().Format("2006-01-02 15:04:05"), this.Name)

	select {
	case <-this.C:
	default:
	}
	close(this.C)
}

// 监听当前User channel的方法，一旦有消息，就直接发送给对端客户端
func (this *User) ListenMessage() {

	for msg := range this.C {
		_, err := this.conn.Write([]byte(msg + "\n"))
		if err != nil {
			fmt.Println("ListenMessage Write err:", err)
			return
		}
	}
}

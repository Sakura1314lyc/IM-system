package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type User struct {
	Name            string
	Addr            string
	C               chan string
	conn            net.Conn
	IsAuthenticated bool
	Avatar          string // 用户头像
	mu              sync.RWMutex

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
	if this.conn == nil {
		return
	}
	_, err := this.conn.Write([]byte(msg))
	if err != nil {
		fmt.Printf("SendMsg error: %v\n", err)
		this.Offline()
	}
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

// 用户处理消息的业务
func (this *User) DoMessage(msg string) {
	if !this.IsAuthenticated {
		if strings.HasPrefix(msg, "login|") {
			parts := strings.Split(msg, "|")
			if len(parts) != 3 {
				this.SendMsg("登录格式错误,示例:login|username|password\n")
				return
			}
			username := strings.TrimSpace(parts[1])
			password := strings.TrimSpace(parts[2])
			if avatar, ok := this.Server.Authenticate(username, password); ok {
				this.Name = username
				this.Avatar = avatar
				this.IsAuthenticated = true
				this.Online()
				this.SendMsg("登录成功\n")
			} else {
				this.SendMsg("用户名或密码错误\n")
			}
		} else {
			this.SendMsg("请先登录,格式:login|username|password\n")
		}
		return
	}

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

	// 处理 rename 命令 - 注意：修复了前面的逻辑混乱问题
	if strings.HasPrefix(lower, "rename|") {
		parts := strings.SplitN(msg, "|", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
			this.SendMsg("rename 命令格式错误,示例:rename|newname\n")
			return
		}

		newName := strings.TrimSpace(parts[1])
		// 修复并发问题：检查和修改需要原子化
		if this.Server.RenameUser(this.Name, newName) {
			this.mu.Lock()
			this.Name = newName
			this.mu.Unlock()
			this.SendMsg("您已成功更新用户名:" + newName + "\n")
		} else {
			this.SendMsg("当前用户名已被使用\n")
		}
		return
	}

	// 处理私聊命令
	if strings.HasPrefix(lower, "to|") {
		parts := strings.SplitN(msg, "|", 3)
		if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
			this.SendMsg("私聊命令格式错误,示例:to|username|hello\n")
			return
		}

		targetName := strings.TrimSpace(parts[1])
		content := strings.TrimSpace(parts[2])

		if err := this.Server.SendPrivate(this.Name, targetName, content, this.Avatar); err != nil {
			this.SendMsg(err.Error() + "\n")
			return
		}

		this.SendMsg("[已发送] " + content + "\n")
		return
	}

	// 处理群聊命令
	if strings.HasPrefix(lower, "group|") {
		parts := strings.SplitN(msg, "|", 4)
		if len(parts) < 2 {
			this.SendMsg("群聊命令格式错误,示例:group|create|群名 或 group|join|群名 或 group|leave|群名 或 group|send|群名|消息\n")
			return
		}
		cmd := strings.TrimSpace(parts[1])
		switch cmd {
		case "create":
			if len(parts) != 3 {
				this.SendMsg("创建群格式:group|create|群名\n")
				return
			}
			groupName := strings.TrimSpace(parts[2])
			if err := this.Server.CreateGroup(groupName, this.Name); err != nil {
				this.SendMsg(err.Error() + "\n")
				return
			}
			this.SendMsg("群组 " + groupName + " 创建成功\n")
		case "join":
			if len(parts) != 3 {
				this.SendMsg("加入群格式:group|join|群名\n")
				return
			}
			groupName := strings.TrimSpace(parts[2])
			if err := this.Server.JoinGroup(groupName, this.Name); err != nil {
				this.SendMsg(err.Error() + "\n")
				return
			}
			this.SendMsg("已加入群组 " + groupName + "\n")
		case "leave":
			if len(parts) != 3 {
				this.SendMsg("离开群格式:group|leave|群名\n")
				return
			}
			groupName := strings.TrimSpace(parts[2])
			if err := this.Server.LeaveGroup(groupName, this.Name); err != nil {
				this.SendMsg(err.Error() + "\n")
				return
			}
			this.SendMsg("已离开群组 " + groupName + "\n")
		case "send":
			if len(parts) != 4 {
				this.SendMsg("群聊发送格式:group|send|群名|消息\n")
				return
			}
			groupName := strings.TrimSpace(parts[2])
			content := strings.TrimSpace(parts[3])
			if err := this.Server.BroadCastToGroup(groupName, this.Name, content, this.Avatar); err != nil {
				this.SendMsg(err.Error() + "\n")
				return
			}
		default:
			this.SendMsg("未知群聊命令\n")
		}
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

	// 安全关闭 channel - 避免在已关闭的 channel 上发送导致 panic
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("关闭 channel 时发生 panic: %v\n", r)
		}
	}()
	
	// 清空缓冲区
	select {
	case <-this.C:
	default:
	}
	close(this.C)
}

// 监听当前User channel的方法，一旦有消息，就直接发送给对端客户端
func (this *User) ListenMessage() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ListenMessage panic: %v\n", r)
		}
	}()

	for msg := range this.C {
		if this.conn == nil {
			return
		}
		_, err := this.conn.Write([]byte(msg + "\n"))
		if err != nil {
			fmt.Printf("ListenMessage Write error: %v\n", err)
			return
		}
	}
}

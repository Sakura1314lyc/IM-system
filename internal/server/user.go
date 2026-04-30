package server

import (
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
	Avatar          string
	mu              sync.RWMutex

	Server *Server
}

func NewUser(conn net.Conn, server *Server) *User {
	userAddr := conn.RemoteAddr().String()
	user := &User{
		Name:   userAddr,
		Addr:   userAddr,
		C:      make(chan string, 100),
		conn:   conn,
		Server: server,
	}

	go user.ListenMessage()

	return user
}

func (u *User) Online() {
	u.Server.mapLock.Lock()
	for key, user := range u.Server.OnlineMap {
		if user == u && key != u.Name {
			delete(u.Server.OnlineMap, key)
		}
	}

	u.Server.OnlineMap[u.Name] = u
	u.Server.mapLock.Unlock()

	fmt.Printf("[%s] 用户 %s(%s) 上线\n", time.Now().Format("2006-01-02 15:04:05"), u.Name, u.Addr)

	u.Server.BroadCast(u, "已上线")
}

func (u *User) SendMsg(msg string) {
	if u.conn == nil {
		return
	}
	_, err := u.conn.Write([]byte(msg))
	if err != nil {
		fmt.Printf("SendMsg error: %v\n", err)
		u.Offline()
	}
}

func (u *User) DoMessage(msg string) {
	if !u.IsAuthenticated {
		if strings.HasPrefix(msg, "login|") {
			parts := strings.Split(msg, "|")
			if len(parts) != 3 {
				u.SendMsg("登录格式错误,示例:login|username|password\n")
				return
			}
			username := strings.TrimSpace(parts[1])
			password := strings.TrimSpace(parts[2])
			if avatar, ok := u.Server.Authenticate(username, password); ok {
				if u.Name != username {
					if !u.Server.RenameUser(u.Name, username) {
						u.SendMsg("登录失败：用户名已被占用\n")
						return
					}
					u.Name = username
				}
				u.Avatar = avatar
				u.IsAuthenticated = true
				u.Online()
				u.SendMsg("登录成功\n")
			} else {
				u.SendMsg("用户名或密码错误\n")
			}
		} else if strings.HasPrefix(msg, "register|") {
			parts := strings.Split(msg, "|")
			if len(parts) != 3 {
				u.SendMsg("注册格式错误,示例:register|username|password\n")
				return
			}
			username := strings.TrimSpace(parts[1])
			password := strings.TrimSpace(parts[2])
			if err := u.Server.Register(username, password, "👤"); err != nil {
				u.SendMsg("注册失败: " + err.Error() + "\n")
			} else {
				u.SendMsg("注册成功，请使用 login|username|password 登录\n")
			}
		} else {
			u.SendMsg("请先登录或注册,格式:login|username|password 或 register|username|password\n")
		}
		return
	}

	msg = strings.TrimSpace(msg)
	lower := strings.ToLower(msg)

	if lower == "who" {
		u.Server.mapLock.RLock()
		for _, user := range u.Server.OnlineMap {
			if user.Name == u.Name {
				continue
			}
			onlineMsg := "[" + user.Addr + "]" + user.Name + ":" + "在线...\n"
			u.SendMsg(onlineMsg)
		}
		u.Server.mapLock.RUnlock()
		return
	}

	if strings.HasPrefix(lower, "rename|") {
		parts := strings.SplitN(msg, "|", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
			u.SendMsg("rename 命令格式错误,示例:rename|newname\n")
			return
		}

		newName := strings.TrimSpace(parts[1])
		if u.Server.RenameUser(u.Name, newName) {
			u.mu.Lock()
			u.Name = newName
			u.mu.Unlock()
			u.SendMsg("您已成功更新用户名:" + newName + "\n")
		} else {
			u.SendMsg("当前用户名已被使用\n")
		}
		return
	}

	if strings.HasPrefix(lower, "to|") {
		parts := strings.SplitN(msg, "|", 3)
		if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
			u.SendMsg("私聊命令格式错误,示例:to|username|hello\n")
			return
		}

		targetName := strings.TrimSpace(parts[1])
		content := strings.TrimSpace(parts[2])

		if err := u.Server.SendPrivate(u.Name, targetName, content, u.Avatar); err != nil {
			u.SendMsg(err.Error() + "\n")
			return
		}

		u.SendMsg("[已发送] " + content + "\n")
		return
	}

	if strings.HasPrefix(lower, "group|") {
		parts := strings.SplitN(msg, "|", 4)
		if len(parts) < 2 {
			u.SendMsg("群聊命令格式错误,示例:group|create|群名 或 group|join|群名 或 group|leave|群名 或 group|send|群名|消息\n")
			return
		}
		cmd := strings.TrimSpace(parts[1])
		switch cmd {
		case "create":
			if len(parts) != 3 {
				u.SendMsg("创建群格式:group|create|群名\n")
				return
			}
			groupName := strings.TrimSpace(parts[2])
			if err := u.Server.CreateGroup(groupName, u.Name); err != nil {
				u.SendMsg(err.Error() + "\n")
				return
			}
			u.SendMsg("群组 " + groupName + " 创建成功\n")
		case "join":
			if len(parts) != 3 {
				u.SendMsg("加入群格式:group|join|群名\n")
				return
			}
			groupName := strings.TrimSpace(parts[2])
			if err := u.Server.JoinGroup(groupName, u.Name); err != nil {
				u.SendMsg(err.Error() + "\n")
				return
			}
			u.SendMsg("已加入群组 " + groupName + "\n")
		case "leave":
			if len(parts) != 3 {
				u.SendMsg("离开群格式:group|leave|群名\n")
				return
			}
			groupName := strings.TrimSpace(parts[2])
			if err := u.Server.LeaveGroup(groupName, u.Name); err != nil {
				u.SendMsg(err.Error() + "\n")
				return
			}
			u.SendMsg("已离开群组 " + groupName + "\n")
		case "send":
			if len(parts) != 4 {
				u.SendMsg("群聊发送格式:group|send|群名|消息\n")
				return
			}
			groupName := strings.TrimSpace(parts[2])
			content := strings.TrimSpace(parts[3])
			if err := u.Server.BroadCastToGroup(groupName, u.Name, content, u.Avatar); err != nil {
				u.SendMsg(err.Error() + "\n")
				return
			}
		default:
			u.SendMsg("未知群聊命令\n")
		}
		return
	}

	u.Server.BroadCast(u, msg)
}

func (u *User) Offline() {
	u.Server.mapLock.Lock()
	for key, user := range u.Server.OnlineMap {
		if user == u {
			delete(u.Server.OnlineMap, key)
		}
	}
	u.Server.mapLock.Unlock()

	u.Server.BroadCast(u, "已下线")

	if u.conn != nil {
		u.conn.Close()
	}

	fmt.Printf("[%s] 用户 %s 离线\n", time.Now().Format("2006-01-02 15:04:05"), u.Name)

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("关闭 channel 时发生 panic: %v\n", r)
		}
	}()

	select {
	case <-u.C:
	default:
	}
	close(u.C)
}

func (u *User) ListenMessage() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ListenMessage panic: %v\n", r)
		}
	}()

	for msg := range u.C {
		if u.conn == nil {
			return
		}
		_, err := u.conn.Write([]byte(msg + "\n"))
		if err != nil {
			fmt.Printf("ListenMessage Write error: %v\n", err)
			return
		}
	}
}

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

type Client struct {
	Serverip   string
	ServerPort int
	Name       string
	conn       net.Conn
	flag       int // 当前client的模式
	reader     *bufio.Reader
	writer     *bufio.Writer
	authenticated bool
}

func NewClient(serverip string, serverport int) *Client {
	c := &Client{
		Serverip:   serverip,
		ServerPort: serverport,
		flag:       99,
	}

	conn, err := net.Dial("tcp", net.JoinHostPort(serverip, fmt.Sprintf("%d", serverport)))
	if err != nil {
		fmt.Println("net.Dial error:", err)
		return nil
	}

	// 启用 TCP Keep-Alive
	tcpConn := conn.(*net.TCPConn)
	tcpConn.SetKeepAlive(true)
	tcpConn.SetKeepAlivePeriod(30 * time.Second)

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.writer = bufio.NewWriter(conn)

	return c
}

func (c *Client) authMenu() bool {
	var choice int

	fmt.Println("\n========== Lchat TCP 客户端 ==========")
	fmt.Println("1. 登录")
	fmt.Println("2. 注册")
	fmt.Println("0. 退出")

	fmt.Print("请选择: ")
	fmt.Scanln(&choice)

	switch choice {
	case 1:
		return c.doLogin()
	case 2:
		return c.doRegister()
	case 0:
		return false
	default:
		fmt.Println(">>>>>请输入合法范围内的数字<<<<<")
		return false
	}
}

func (c *Client) doLogin() bool {
	var username, password string

	fmt.Print("用户名: ")
	fmt.Scanln(&username)
	fmt.Print("密码: ")
	fmt.Scanln(&password)

	msg := fmt.Sprintf("login|%s|%s\n", username, password)
	_, err := c.writer.WriteString(msg)
	if err != nil {
		fmt.Println("发送失败:", err)
		return false
	}
	c.writer.Flush()

	// 读取服务器响应
	resp, err := c.reader.ReadString('\n')
	if err != nil {
		fmt.Println("读取响应失败:", err)
		return false
	}
	resp = strings.TrimSpace(resp)
	fmt.Println(resp)

	if strings.Contains(resp, "登录成功") {
		c.Name = username
		c.authenticated = true
		return true
	}
	return false
}

func (c *Client) doRegister() bool {
	var username, password string

	fmt.Print("用户名: ")
	fmt.Scanln(&username)
	fmt.Print("密码(至少6位): ")
	fmt.Scanln(&password)

	msg := fmt.Sprintf("register|%s|%s\n", username, password)
	_, err := c.writer.WriteString(msg)
	if err != nil {
		fmt.Println("发送失败:", err)
		return false
	}
	c.writer.Flush()

	resp, err := c.reader.ReadString('\n')
	if err != nil {
		fmt.Println("读取响应失败:", err)
		return false
	}
	resp = strings.TrimSpace(resp)
	fmt.Println(resp)

	return strings.Contains(resp, "注册成功")
}

func (c *Client) menu() bool {
	var flag int

	fmt.Println("\n========== Lchat 主菜单 ==========")
	fmt.Println("1. 公聊模式")
	fmt.Println("2. 私聊模式")
	fmt.Println("3. 群聊操作")
	fmt.Println("4. 更新用户名")
	fmt.Println("5. 查看在线用户")
	fmt.Println("0. 退出")

	fmt.Print("请选择: ")
	fmt.Scanln(&flag)
	if flag >= 0 && flag <= 5 {
		c.flag = flag
		return true
	}
	fmt.Println(">>>>>请输入合法范围内的数字<<<<<")
	return false
}

func (c *Client) PublicChat() {
	var chatMsg string
	fmt.Println(">>>>>输入聊天内容, exit退出.")
	fmt.Scanln(&chatMsg)

	for chatMsg != "exit" {
		if len(chatMsg) != 0 {
			_, err := c.writer.WriteString(chatMsg + "\n")
			if err != nil {
				fmt.Println("conn Write err:", err)
				break
			}
			c.writer.Flush()
		}

		chatMsg = ""
		fmt.Println(">>>>>请输入聊天内容,exit退出.")
		fmt.Scanln(&chatMsg)
	}
}

func (c *Client) SearchUsers() {
	c.writer.WriteString("who\n")
	c.writer.Flush()
}

func (c *Client) Privatechat() {
	var RemoteName string
	var ChatMsg string

	c.SearchUsers()
	fmt.Println(">>>>>请输入聊天对象[用户名], exit退出.")
	fmt.Scanln(&RemoteName)

	for RemoteName != "exit" {
		fmt.Println(">>>>>请输入消息内容,exit退出:")
		fmt.Scanln(&ChatMsg)

		for ChatMsg != "exit" {
			if len(ChatMsg) != 0 {
				sendMsg := "to|" + RemoteName + "|" + ChatMsg + "\n"
				_, err := c.writer.WriteString(sendMsg)
				if err != nil {
					fmt.Println("conn Write err:", err)
					break
				}
				c.writer.Flush()
			}

			ChatMsg = ""
			fmt.Println(">>>>>请输入消息内容,exit退出:")
			fmt.Scanln(&ChatMsg)
		}

		c.SearchUsers()
		fmt.Println(">>>>>请输入聊天对象[用户名], exit退出.")
		fmt.Scanln(&RemoteName)
	}
}

func (c *Client) GroupChat() {
	var action, groupName, content string

	fmt.Println(">>>>>群聊操作: create / join / leave / send")
	fmt.Print("操作: ")
	fmt.Scanln(&action)

	switch action {
	case "create":
		fmt.Print("群名: ")
		fmt.Scanln(&groupName)
		c.writer.WriteString(fmt.Sprintf("group|create|%s\n", groupName))
	case "join":
		fmt.Print("群名: ")
		fmt.Scanln(&groupName)
		c.writer.WriteString(fmt.Sprintf("group|join|%s\n", groupName))
	case "leave":
		fmt.Print("群名: ")
		fmt.Scanln(&groupName)
		c.writer.WriteString(fmt.Sprintf("group|leave|%s\n", groupName))
	case "send":
		fmt.Print("群名: ")
		fmt.Scanln(&groupName)
		fmt.Print("消息: ")
		fmt.Scanln(&content)
		c.writer.WriteString(fmt.Sprintf("group|send|%s|%s\n", groupName, content))
	default:
		fmt.Println("未知操作")
		return
	}
	c.writer.Flush()
}

func (c *Client) UpdateName() bool {
	fmt.Println(">>>>>请输入用户名:")
	fmt.Scanln(&c.Name)

	sendMsg := "rename|" + c.Name + "\n"
	_, err := c.writer.WriteString(sendMsg)
	if err != nil {
		fmt.Println("conn.Write err:", err)
		return false
	}
	c.writer.Flush()
	return true
}

func (c *Client) DealResponse() {
	io.Copy(os.Stdout, c.conn)
}

func (c *Client) Run() {
	// 先认证
	for !c.authenticated {
		if !c.authMenu() {
			fmt.Println(">>>>>认证失败或退出连接.")
			return
		}
	}

	// 启动后台 goroutine 接收服务器消息
	go c.DealResponse()

	for c.flag != 0 {
		for c.menu() != true {
		}

		switch c.flag {
		case 1:
			c.PublicChat()
		case 2:
			c.Privatechat()
		case 3:
			c.GroupChat()
		case 4:
			c.UpdateName()
		case 5:
			c.SearchUsers()
		}
	}
}

var serverIp string
var serverPort int

func init() {
	flag.StringVar(&serverIp, "ip", "127.0.0.1", "设置服务器IP地址(默认为127.0.0.1)")
	flag.IntVar(&serverPort, "port", 8888, "设置服务器端口(默认为8888)")
}

func main() {
	flag.Parse()

	c := NewClient(serverIp, serverPort)
	if c == nil {
		fmt.Println(">>>>>连接服务器失败...")
		return
	}

	fmt.Println(">>>>>连接服务器成功...")
	c.Run()
}

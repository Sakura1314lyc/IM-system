package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
)

type Client struct {
	Serverip   string
	ServerPort int
	Name       string
	conn       net.Conn
	flag       int //表示当前client的模式
}

func NewClient(serverip string, serverport int) *Client {
	//创建用户端对象
	c := &Client{
		Serverip:   serverip,
		ServerPort: serverport,
		flag:       99,
	}

	//链接server
	conn, err := net.Dial("tcp", net.JoinHostPort(serverip, fmt.Sprintf("%d", serverport)))
	if err != nil {
		fmt.Println("net.Dial error", err)
		return nil
	}

	c.conn = conn

	//返回对象
	return c
}

func (c *Client) menu() bool {
	var flag int

	fmt.Println("1.公聊模式")
	fmt.Println("2.私聊模式")
	fmt.Println("3.更新用户名")
	fmt.Println("0.退出")

	fmt.Scanln(&flag)
	if flag >= 0 && flag <= 3 {
		c.flag = flag
		return true
	} else {
		fmt.Println(">>>>>请输入合法范围内的数字<<<<<")
		return false
	}
}

func (c *Client) PublicChat() {
	//提示用户输入消息
	var chatMsg string
	fmt.Println(">>>>>输入聊天内容, exit退出.")
	fmt.Scanln(&chatMsg)

	for chatMsg != "exit" {
		//发给服务器

		//消息不为空则发送
		if len(chatMsg) != 0 {
			sendMsg := chatMsg + "\n"
			_, err := c.conn.Write([]byte(sendMsg))
			if err != nil {
				fmt.Println("conn Write err :", err)
				break
			}
		}

		chatMsg = ""
		fmt.Println(">>>>>请输入聊天内容,exit退出.")
		fmt.Scanln(&chatMsg)
	}

}

// 查询在线用户
func (c *Client) SearchUsers() {
	sendMsg := "who\n"
	_, err := c.conn.Write([]byte(sendMsg))
	if err != nil {
		fmt.Println("conn Write err:", err)
		return
	}
}

// 私聊模式
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
			//消息不为空则发送
			if len(ChatMsg) != 0 {

				sendMsg := "to|" + RemoteName + "|" + ChatMsg + "\n"
				_, err := c.conn.Write([]byte(sendMsg))
				if err != nil {
					fmt.Println("conn Write err:", err)
					break
				}
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

func (c *Client) UpdateName() bool {

	fmt.Println(">>>>>请输入用户名:")
	fmt.Scanln(&c.Name)

	sendMsg := "rename|" + c.Name + "\n"
	_, err := c.conn.Write([]byte(sendMsg))
	if err != nil {
		fmt.Println("conn.Write err:", err)
		return false
	}
	return true
}

// 处理server回应的消息，直接显示到标准输出即可
func (c *Client) DealResponse() {
	//一旦c.conn 有数据，就直接copy到stdout标准输出上，永久阻塞监听
	io.Copy(os.Stdout, c.conn)
}
func (c *Client) Run() {
	for c.flag != 0 {
		for c.menu() != true {

		}

		//根据不同模式处理不同的业务
		switch c.flag {
		case 1:
			//公聊模式
			c.PublicChat()
		case 2:
			//私聊模式
			c.Privatechat()
		case 3:
			//更新用户名
			c.UpdateName()
			//go的switch默认break
		}
	}
}

var serverIp string
var serverPort int

// ./client -ip 127.0.0.1 -port 8888
func init() {
	flag.StringVar(&serverIp, "ip", "127.0.0.1", "设置服务器IP地址(默认为127.0.0.1)")
	flag.IntVar(&serverPort, "port", 8888, "设置服务器端口(默认为8888)")
}

func main() {
	//命令行解析
	flag.Parse()

	c := NewClient(serverIp, serverPort)
	if c == nil {
		fmt.Println(">>>>>连接服务器失败...")
		return
	}

	//单独开启一个goroutine处理server的回执消息
	go c.DealResponse()

	fmt.Println(">>>>>连接服务器成功...")

	//启动客户端业务
	c.Run()
}

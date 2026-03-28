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
	client := &Client{
		Serverip:   serverip,
		ServerPort: serverport,
		flag:       99,
	}

	//链接server
	conn, err := net.Dial("tcp", fmt.Sprintf("%s : %d", serverip, serverport))
	if err != nil {
		fmt.Println("net.Dial error", err)
		return nil
	}

	client.conn = conn

	//返回对象
	return client
}

func (client *Client) menu() bool {
	var flag int

	fmt.Println("1.公聊模式")
	fmt.Println("2.私聊模式")
	fmt.Println("3.更新用户名")
	fmt.Println("0.退出")

	fmt.Scanln(&flag)
	if flag >= 0 && flag <= 3 {
		client.flag = flag
		return true
	} else {
		fmt.Println(">>>>>请输入合法范围内的数字<<<<<")
		return false
	}
}

func (client *Client) PublicChat() {
	//提示用户输入消息
	var chatMsg string
	fmt.Println(">>>>>输入聊天内容, exit退出.")
	fmt.Scanln(&chatMsg)

	for chatMsg != "exit" {
		//发给服务器

		//消息不为空则发送
		if len(chatMsg) != 0 {
			sendMsg := chatMsg + "\n"
			_, err := client.conn.Write([]byte(sendMsg))
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
func (client *Client) SearchUsers() {
	sendMsg := "who\n"
	_, err := client.conn.Write([]byte(sendMsg))
	if err != nil {
		fmt.Println("conn Write err:", err)
		return
	}
}

// 私聊模式
func (client *Client) Privatechat() {
	var RemoteName string
	var ChatMsg string

	client.SearchUsers()
	fmt.Println(">>>>>请输入聊天对象[用户名], exit退出.")
	fmt.Scanln(&RemoteName)

	for RemoteName != "exit" {
		fmt.Println(">>>>>请输入消息内容,exit退出:")
		fmt.Scanln(&ChatMsg)

		for ChatMsg != "exit" {
			//消息不为空则发送
			if len(ChatMsg) != 0 {

				sendMsg := "to|" + RemoteName + "|" + ChatMsg + "\n\n"
				_, err := client.conn.Write([]byte(sendMsg))
				if err != nil {
					fmt.Println("conn Write err:", err)
					break
				}
			}

			ChatMsg = ""
			fmt.Println(">>>>>请输入消息内容,exit退出:")
			fmt.Scanln(&ChatMsg)
		}

		client.SearchUsers()
		fmt.Println(">>>>>请输入聊天对象[用户名], exit退出.")
		fmt.Scanln(&RemoteName)

	}
}

func (client *Client) UpdateName() bool {

	fmt.Println(">>>>>请输入用户名:")
	fmt.Scanln(&client.Name)

	sendMsg := "rename|" + client.Name + "\n"
	_, err := client.conn.Write([]byte(sendMsg))
	if err != nil {
		fmt.Println("conn.Write err:", err)
		return false
	}
	return true
}

// 处理server回应的消息，直接显示到标准输出即可
func (client *Client) DealResponse() {
	//一旦client.conn 有数据，就直接copy到stdout标准输出上，永久阻塞监听
	io.Copy(os.Stdout, client.conn)
}
func (client *Client) Run() {
	for client.flag != 0 {
		for client.menu() != true {

		}

		//根据不同模式处理不同的业务
		switch client.flag {
		case 1:
			//公聊模式
			client.PublicChat()
		case 2:
			//私聊模式
			client.Privatechat()
		case 3:
			//更新用户名
			client.UpdateName()
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

	client := NewClient(serverIp, serverPort)
	if client == nil {
		fmt.Println(">>>>>连接服务器失败...")
		return
	}

	//单独开启一个goroutine处理server的回执消息
	go client.DealResponse()

	fmt.Println(">>>>>连接服务器成功...")

	//启动客户端业务
	client.Run()
}

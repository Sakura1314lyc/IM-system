package main

import (
	"flag"
	"fmt"
	"net"
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
func (client *Client) Run() {
	for client.flag != 0 {
		for client.menu() != true {

		}

		//根据不同模式处理不同的业务
		switch client.flag {
		case 1:
			//公聊模式
			fmt.Println("进入公聊模式...")
		case 2:
			//私聊模式
			fmt.Println("进入私聊模式...")
		case 3:
			//更新用户名
			fmt.Println("更新用户名...")
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

	fmt.Println(">>>>>连接服务器成功...")

	//启动客户端业务
	client.Run()
}

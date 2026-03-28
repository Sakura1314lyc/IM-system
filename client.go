package main

import (
	"fmt"
	"net"
)

type Client struct {
	Serverip   string
	ServerPort int
	Name       string
	conn       net.Conn
}

func NewClient(serverip string, serverport int) *Client {
	//创建用户端对象
	client := &Client{
		Serverip:   serverip,
		ServerPort: serverport,
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

func main() {
	client := NewClient("127.0.0.1", 8888)
	if client == nil {
		fmt.Println(">>>>>连接服务器失败...")
		return
	}

	fmt.Println(">>>>>连接服务器成功...")

	//启动客户端业务
	select {}
}

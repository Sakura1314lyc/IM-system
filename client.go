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
	select {}
}

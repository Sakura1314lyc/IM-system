package main

import (
	"flag"
	"fmt"
)

var (
	serverIP   string
	serverPort int
	webAddr    string
)

func init() {
	flag.StringVar(&serverIP, "ip", "127.0.0.1", "TCP 服务 IP(默认 127.0.0.1)")
	flag.IntVar(&serverPort, "port", 8888, "TCP 服务端口（默认 8888)")
	flag.StringVar(&webAddr, "web", ":8080", "Web UI 服务地址（默认 :8080)")
}

func main() {
	flag.Parse()

	server := Newserver(serverIP, serverPort)

	// 后端 TCP IM 服务
	go server.Start()

	// Web UI + REST/SSE 服务
	go server.StartWeb(webAddr)

	fmt.Printf("启动完成:TCP %s:%d,Web %s\n", serverIP, serverPort, webAddr)
	select {}
}

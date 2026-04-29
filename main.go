package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

var (
	serverIP   string
	serverPort int
	webAddr    string
	dbPath     string
	enableTLS  bool
)

func init() {
	flag.StringVar(&serverIP, "ip", "127.0.0.1", "TCP 服务 IP(默认 127.0.0.1)")
	flag.IntVar(&serverPort, "port", 8888, "TCP 服务端口（默认 8888)")
	flag.StringVar(&webAddr, "web", ":8080", "Web UI 服务地址（默认 :8080)")
	flag.StringVar(&dbPath, "db", "im.db", "数据库文件路径（默认 im.db）")
	flag.BoolVar(&enableTLS, "tls", false, "启用TLS加密（需要server.crt和server.key文件）")
}

func main() {
	flag.Parse()

	server := NewServer(serverIP, serverPort, dbPath, enableTLS)

	// 后端 TCP IM 服务
	go server.Start()

	// Web UI + REST/SSE 服务
	go server.StartWeb(webAddr)

	fmt.Printf("启动中:TCP %s:%d,Web %s, DB: %s, TLS: %v\n", serverIP, serverPort, webAddr, dbPath, enableTLS)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println("\n正在关闭服务器...")
	server.Shutdown()
	fmt.Println("服务器已关闭")
}

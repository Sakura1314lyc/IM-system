package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"IM-system/internal/config"
	"IM-system/internal/server"
)

func main() {
	env := flag.String("env", "dev", "运行环境 (dev/prod/test)")
	serverIP := flag.String("ip", "127.0.0.1", "TCP 服务 IP(默认 127.0.0.1)")
	serverPort := flag.Int("port", 8888, "TCP 服务端口（默认 8888)")
	webAddr := flag.String("web", ":8080", "Web UI 服务地址（默认 :8080)")
	dbPath := flag.String("db", "im.db", "数据库文件路径（默认 im.db）")
	enableTLS := flag.Bool("tls", false, "启用TLS加密（需要server.crt和server.key文件）")
	flag.Parse()

	cfg, err := config.Load(*env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "配置加载失败: %v\n", err)
		os.Exit(1)
	}

	// Explicitly passed flags override config file values.
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "ip":
			cfg.Server.IP = *serverIP
		case "port":
			cfg.Server.Port = *serverPort
		case "web":
			cfg.Web.Addr = *webAddr
		case "db":
			cfg.DB.Path = *dbPath
		case "tls":
			cfg.Server.TLS = *enableTLS
		}
	})

	srv := server.New(cfg)

	go srv.Start()
	go srv.StartWeb()

	fmt.Printf("启动中 [%s]:TCP %s:%d,Web %s, DB: %s, TLS: %v\n",
		*env, cfg.Server.IP, cfg.Server.Port, cfg.Web.Addr, cfg.DB.Path, cfg.Server.TLS)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println("\n正在关闭服务器...")
	srv.Shutdown()
	fmt.Println("服务器已关闭")
}

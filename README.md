# IM-system

一个基于 Go（TCP）的轻量级即时通信系统示例，包含服务端与命令行客户端。

> 适合用于学习：并发连接处理、在线用户管理、广播/私聊消息分发、简单命令协议设计。

## 项目概览

本项目当前由多个核心模块组成：

- `main.go`：服务端启动入口（含 TCP IM 服务与 Web REST/SSE 服务）。
- `server.go`：服务端核心逻辑（监听连接、转发广播、超时踢出、Web API）。
- `user.go`：用户模型与命令处理（`who` / `rename|` / `to|`）。
- `cmd/client/main.go`：命令行客户端（公聊、私聊、改名）。
- `web/`：前端 UI 原型，包含 `index.html`、`styles.css`、`app.js`。

## 功能清单（与当前代码一致）

- ✅ TCP 长连接聊天（非 WebSocket）。
- ✅ 在线用户上线/下线广播。
- ✅ 公聊（广播到所有在线用户）。
- ✅ 私聊（`to|用户名|消息`）。
- ✅ 查询在线用户（`who`）。
- ✅ 修改用户名（`rename|新用户名`）。
- ✅ 服务端空闲超时踢出（10 分钟无消息）。
- ✅ 基于 `goroutine + channel` 的并发消息处理。

## 环境要求

- Go 1.20+（建议）
- Linux / macOS / Windows（可运行 Go 即可）

## 快速开始

### 1）启动服务端

在项目根目录执行：

```bash
go run .
```

或者（显式传入文件，避免漏掉 `server.go`/`user.go`）：

```bash
go run main.go server.go user.go
```

默认服务地址：

- TCP IM 服务：`127.0.0.1:8888`（老版命令行客户端互通）。
- Web UI 服务：`:8080`（浏览器访问 `http://127.0.0.1:8080/`）。

支持参数：

```bash
go run . -ip 127.0.0.1 -port 8888 -web :8080
```

> 说明：`main.go` 同时启动了 TCP 服务与 Web 服务，`server.go`、`user.go` 提供了核心 IM 逻辑。

---

### 2）启动客户端

新开一个终端窗口：

```bash
go run ./cmd/client/main.go -ip 127.0.0.1 -port 8888
```

可再开多个终端重复执行上面命令，模拟多个用户在线。

## 客户端使用说明

连接成功后会显示菜单：

- `1` 公聊模式
- `2` 私聊模式
- `3` 更新用户名
- `0` 退出

### 常用协议命令

客户端内部会自动组装命令；如果你用 `nc/telnet` 手动测试，可直接发送：

- 查询在线用户：

```text
who
```

- 修改用户名：

```text
rename|alice
```

- 私聊：

```text
to|alice|你好
```

## 运行示例（建议验证流程）

1. 启动服务端。
2. 启动两个客户端 A、B。
3. A 改名为 `alice`，B 改名为 `bob`。
4. A 在公聊发送消息，确认 B 能收到。
5. B 使用私聊给 A 发送消息，确认 A 能收到。
6. 在任一客户端发送 `who`，确认能看到在线用户列表。

## 并发与实现说明

- 服务端通过 `OnlineMap` 维护在线用户，并使用 `RWMutex` 保护并发读写。
- 广播通过 `Server.Message` channel 分发。
- 每个用户有独立消息 channel（`User.C`）与写回协程。
- 为避免慢客户端阻塞，广播发送使用了非阻塞写（`select + default`）。

## 已知限制 / 后续优化建议

当前仓库定位为学习示例，可按需扩展：

- [ ] 增加消息持久化（MySQL / Redis / Kafka 等）。
- [ ] 增加认证鉴权（token / JWT）。
- [ ] 增加房间（群组）与历史消息拉取。
- [ ] 提升协议鲁棒性（粘包拆包、统一序列化格式，如 JSON/Protobuf）。
- [ ] 完善单元测试与集成测试。


## Web UI 已接入后端（支持前后端实时连通）

新的 `main.go` 已集成 Web API + SSE，前端可直接与后端交互：

- `GET /api/events?name=<昵称>`：SSE 实时接收消息
- `GET /api/online`：获取当前在线用户列表
- `POST /api/send`：发送消息（支持公聊/私聊）
- `POST /api/rename`：修改昵称

静态页面已被放在 `web/` 目录：

- `web/index.html`：页面结构
- `web/styles.css`：界面风格增强
- `web/app.js`：改为真实 API 调用 + 响应式消息流

### 访问步骤

1. 运行服务：`go run main.go server.go user.go`
2. 打开浏览器：`http://127.0.0.1:8080/`
3. 在“昵称”输入框设置用户名，点击“修改”。
4. 切换公聊/私聊，输入消息发送。

> 注：TCP CLI 用户（`go run ./cmd/client/main.go`）与 Web UI 用户可混合在线互通。


### 截图说明（browser_container 不可用时）

如果你的 Agent 运行环境没有提供 `browser_container` 工具（例如仅有 shell），可以用以下方式替代：

1. 本地直接访问 `http://127.0.0.1:8080/web/`（先执行 `python3 -m http.server 8080`）。
2. 使用系统截图工具手动截取页面。
3. 如果仓库已安装 Playwright/Puppeteer，可写自动化脚本产出截图；若未安装，不建议在受限环境中临时安装。

> 结论：`browser_container` 不是项目内配置项，而是平台提供的能力；项目侧无法“开启”它，只能在支持该工具的平台会话中使用。

## 测试

```bash
go test ./...
```

> 当前仓库可能尚未包含完整测试用例，建议以多客户端联调为主。

## License

默认可按 MIT License 使用（如需发布请补充 `LICENSE` 文件）。

# IM-system

一个基于 Go 的轻量级即时通讯系统，支持 **TCP 命令行客户端** 和 **Web UI** 双端接入，后端集成 SQLite 持久化、REST API、SSE 实时推送。

## 项目结构

```
├── main.go              # 启动入口（TCP + Web 双服务 + 优雅关闭）
├── server.go            # 服务端核心（网络层、业务逻辑、REST API、中间件、TLS）
├── user.go              # 用户模型与 TCP 命令处理
├── db.go                # SQLite 数据库层（用户、群组、消息、好友）
├── cmd/
│   ├── client/main.go   # TCP 命令行客户端
│   └── web/main.go      # Web 网关代理（可选独立部署）
└── web/                 # 前端 UI
    ├── index.html
    ├── styles.css
    └── app.js
```

## 功能清单

- ✅ TCP 长连接 + Web UI 双端接入，用户互通
- ✅ 用户注册/登录，密码 bcrypt 哈希存储
- ✅ 公聊广播、私聊（`to|用户名|消息`）、群聊（创建/加入/离开/发送）
- ✅ 好友系统（双向关系，事务保证）
- ✅ 在线用户查询、改名
- ✅ 消息历史持久化（SQLite，支持按类型/私聊/群聊查询）
- ✅ Web UI 基于 SSE 实时推送，支持头像、个人资料
- ✅ 认证中间件（减少 handler 重复代码）
- ✅ 服务端空闲超时踢出（10 分钟无消息）
- ✅ 输入安全过滤（防 XSS）
- ✅ 可选 TLS 加密（自动生成 ECDSA 自签名证书）
- ✅ 端口被占用时自动回退
- ✅ 优雅关闭（SIGINT/SIGTERM 信号处理）
- ✅ IPv6 兼容

## 环境要求

- Go 1.20+
- 运行于 Linux / macOS / Windows

## 快速开始

### 启动服务端

```bash
go run .
```

默认地址：
- TCP IM 服务：`127.0.0.1:8888`
- Web UI 服务：`http://127.0.0.1:8080/`

支持参数：

```bash
go run . -ip 127.0.0.1 -port 8888 -web :8080 -db im.db -tls
```

> `-tls` 会自动生成 ECDSA P256 自签名证书用于开发环境，生产环境请使用正规证书。

### 启动 TCP 客户端

```bash
go run ./cmd/client/main.go -ip 127.0.0.1 -port 8888
```

连接后先登录或注册：

```text
login|alice|123456
```

默认用户：`alice` / `bob` / `charlie`，密码均为 `123456`。

### 访问 Web UI

浏览器打开 `http://127.0.0.1:8080/`，注册新用户或使用默认用户登录。

## 客户端使用

### TCP 命令

| 命令 | 格式 | 说明 |
|------|------|------|
| 注册 | `register\|用户名\|密码` | 密码至少 6 位 |
| 登录 | `login\|用户名\|密码` | |
| 在线用户 | `who` | 列出其他在线用户 |
| 改名 | `rename\|新用户名` | |
| 私聊 | `to\|用户名\|消息` | |
| 创建群 | `group\|create\|群名` | |
| 加入群 | `group\|join\|群名` | |
| 发群消息 | `group\|send\|群名\|消息` | |

### Web API

所有 GET 请求通过 `?token=` 鉴权，POST 请求通过请求体 `token` 字段鉴权。

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/login` | 登录获取 token |
| POST | `/api/register` | 注册新用户 |
| GET | `/api/events?token=` | SSE 实时消息流 |
| GET | `/api/online` | 在线用户（无需认证） |
| POST | `/api/send` | 发送消息（公聊/私聊/群聊） |
| GET | `/api/history?token=&type=&peer=&group=` | 历史消息 |
| GET | `/api/groups?token=` | 我的群组列表 |
| POST | `/api/group` | 群组操作（create/join/leave/invite/kick） |
| GET | `/api/friends?token=` | 好友列表 |
| POST | `/api/friend` | 添加/删除好友 |
| GET | `/api/profile?token=&user=` | 查看个人资料 |
| POST | `/api/profile` | 更新个人资料 |
| POST | `/api/avatar` | 更新头像 |
| POST | `/api/rename` | 修改昵称 |

## 架构要点

- **并发模型**：每个 TCP 连接一个 goroutine，消息通过 `channel` 分发
- **认证中间件**：GET 路由统一使用 `authQueryMiddleware`，从 context 取用户身份
- **数据库**：SQLite，使用参数化查询防 SQL 注入，事务保证好友关系一致性
- **TLS**：`-tls` 标志启动时自动生成 ECDSA 自签名证书，无需手动配置
- **优雅关闭**：捕获 SIGINT/SIGTERM，关闭 listener 释放端口

## License

MIT

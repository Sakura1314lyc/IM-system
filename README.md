# Lchat - 轻量级即时通讯系统

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green)
![CI](https://img.shields.io/badge/CI-GitHub%20Actions-2088FF?logo=githubactions&logoColor=white)
![Coverage](https://img.shields.io/badge/Coverage-82%25-89C35B)

一个基于 Go 的轻量级即时通讯系统，支持 **TCP 命令行客户端** 和 **Web UI** 双端接入。后端集成 SQLite 持久化、REST API、WebSocket 实时推送，用户数据互通。

## 项目结构

```
├── main.go                  # 启动入口（TCP + Web 双服务 + 优雅关闭）
├── internal/
│   ├── config/              # 配置管理（JSON + 环境变量覆盖）
│   │   ├── config.go
│   │   └── config_test.go
│   ├── model/model.go       # 数据模型（User, Message, Group, Session 等）
│   ├── db/db.go             # SQLite 数据库层（参数化查询，事务保证）
│   └── server/
│       ├── server.go        # TCP 监听、消息广播、会话管理
│       ├── user.go          # 用户模型与 TCP 命令处理
│       ├── web.go           # REST API、HTTP 中间件、TLS 证书
│       ├── wss.go           # WebSocket 处理器与 WSClient
│       ├── bus.go           # SessionStore / MessageBus 接口定义
│       ├── redis_session.go # Redis 会话存储实现
│       ├── redis_bus.go     # Redis Pub/Sub 消息总线实现
│       ├── avatar.go        # 头像存储（文件系统 + emoji）
│       ├── storage.go       # Storage 接口定义
│       ├── ratelimit.go     # 速率限制器
│       ├── bench_test.go    # 性能基准测试
│       ├── server_test.go   # 服务端单元测试
│       ├── redis_session_test.go  # Redis 会话存储测试
│       ├── redis_bus_test.go      # Redis 消息总线测试
│       └── web_test.go      # HTTP handler 测试
├── cmd/
│   ├── client/main.go       # TCP 命令行客户端
│   └── web/main.go          # Web 网关代理（可选独立部署）
├── web/                     # 前端 UI
│   ├── index.html
│   ├── styles.css
│   └── app.js
├── config/                  # 多环境配置文件
│   ├── dev.json
│   ├── prod.json
│   └── test.json
└── uploads/                 # 头像文件存储目录
```

## 功能清单

- ✅ TCP 长连接 + Web UI 双端接入，用户互通
- ✅ 用户注册/登录，密码 bcrypt 哈希存储
- ✅ 公聊广播、私聊、群聊（创建/加入/离开/发送）
- ✅ 好友系统（双向关系，事务保证）
- ✅ 在线用户查询、改名
- ✅ 消息历史持久化（SQLite，支持按类型/私聊/群聊查询）
- ✅ Web UI 基于 WebSocket 双向实时通信
- ✅ 头像支持 emoji 和图片上传（文件存储）
- ✅ 会话 Token 自动过期 + 后台定期清理
- ✅ 登录/注册/消息发送速率限制（基于 IP）
- ✅ 服务端空闲超时踢出
- ✅ 输入安全过滤（防 XSS + 长度截断）
- ✅ 可选 TLS 加密（自动生成 ECDSA 自签名证书）
- ✅ 端口被占用时自动回退
- ✅ 优雅关闭（SIGINT/SIGTERM 信号处理）
- ✅ IPv6 兼容
- ✅ 多环境配置管理（dev/prod/test JSON + 环境变量覆盖）
- ✅ 接口化存储层（`Storage` 接口，方便测试与替换）
- ✅ 结构化日志（`log/slog`，支持 JSON 输出）
- ✅ CORS 跨域支持
- ✅ WebSocket 自动重连（指数退避）
- ✅ 离线消息处理（消息存入数据库，上线后加载）
- ✅ Redis 会话存储（水平扩展，多实例共享会话）
- ✅ Redis 消息总线（Pub/Sub 跨服务器消息分发）
- ✅ Docker Compose 一键部署（Redis + 应用）
- ✅ 深色模式 / 字体大小 / 消息声音等个性化设置
- ✅ 好友动态（想法发布与评论）
- ✅ 全面的单元测试覆盖

## 环境要求

- Go 1.21+
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
# 指定运行环境（dev/prod/test），默认 dev
go run . -env prod

# 环境变量覆盖配置（IM_ 前缀）
IM_SERVER_PORT=9999 IM_SESSION_TTL=12h go run .

# 传统参数仍可用，会覆盖配置文件对应值
go run . -ip 127.0.0.1 -port 8888 -tls
```

> `-tls` 会自动生成 ECDSA P256 自签名证书用于开发环境，生产环境请使用正规证书。

### Docker 部署（含 Redis）

```bash
# 启动 Redis + 应用服务（自动使用 Redis 会话存储和消息总线）
docker-compose up --build
```

默认地址：
- Web UI 服务：`http://127.0.0.1:8080/`
- Redis 服务：`redis:6379`（容器内）

> Redis 模式启用后，多台服务器实例共享会话状态，消息通过 Redis Pub/Sub 跨服务器分发，支持水平扩展。

### 多环境配置

项目使用 `config/{env}.json` 加载配置，加载优先级：**内置默认值 ← 配置文件 ← 环境变量 ← 命令行参数**。

```bash
go run . -env dev    # 加载 config/dev.json （开发，默认）
go run . -env prod   # 加载 config/prod.json（生产：TLS 开启、0.0.0.0）
go run . -env test   # 加载 config/test.json（测试：内存 DB、短超时）
```

支持的环境变量（`IM_` 前缀）：

| 变量 | 作用 | 示例 |
|------|------|------|
| `IM_SERVER_IP` | TCP 监听地址 | `0.0.0.0` |
| `IM_SERVER_PORT` | TCP 端口 | `8888` |
| `IM_SERVER_TLS` | 启用 TLS | `true` |
| `IM_WEB_ADDR` | Web 监听地址 | `:8080` |
| `IM_DB_PATH` | 数据库路径 | `data/im.db` |
| `IM_SESSION_TTL` | 会话有效期 | `24h` |
| `IM_SERVER_IDLE_TIMEOUT` | 用户超时踢出 | `10m` |
| `IM_RATE_LIMIT` | 每分钟最大请求数 | `30` |
| `IM_MAX_MSG_LENGTH` | 消息最大长度 | `2000` |
| `IM_UPLOAD_DIR` | 头像上传目录 | `uploads` |
| `IM_SESSION_BACKEND` | 会话存储后端 | `memory` / `redis` |
| `IM_REDIS_ADDR` | Redis 地址 | `localhost:6379` |
| `IM_REDIS_PASSWORD` | Redis 密码 | |
| `IM_REDIS_DB` | Redis 数据库号 | `0` |

### 启动 TCP 客户端

```bash
go run ./cmd/client/main.go -ip 127.0.0.1 -port 8888
```

连接后先登录或注册：

```
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

所有请求均支持 `Authorization: Bearer <token>` 头部鉴权，同时也兼容 `?token=` 查询参数和请求体 `token` 字段。

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/login` | 登录获取 token |
| POST | `/api/register` | 注册新用户（支持上传头像） |
| GET | `/api/ws` | WebSocket 实时消息（先建立连接，再发送 JSON 认证） |
| GET | `/api/online` | 在线用户（无需认证） |
| POST | `/api/send` | 发送消息（公聊/私聊/群聊） |
| GET | `/api/history` | 历史消息查询 |
| GET | `/api/groups` | 我的群组列表 |
| POST | `/api/group` | 群组操作（create/join/leave/invite） |
| GET | `/api/friends` | 好友列表 |
| POST | `/api/friend` | 添加/删除好友 |
| GET | `/api/profile` | 查看个人资料 |
| POST | `/api/profile` | 更新个人资料 |
| POST | `/api/avatar` | 更新头像 |
| POST | `/api/rename` | 修改昵称 |

## 架构要点

- **分层设计**：`model` → `db`（实现 `Storage` 接口） → `server` 三层依赖，无循环引用
- **存储层接口**：`Storage` 接口抽象数据库操作，可替换为不同后端，也便于单元测试 mock
- **双端用户互通**：TCP 连接和 WebSocket 连接共享 `OnlineMap` / `WSConns`，公聊消息互通
- **并发模型**：每个 TCP/WebSocket 连接一个 goroutine，消息通过有缓冲 channel 分发，读写分离
- **认证中间件**：GET 路由使用 `authQueryMiddleware` 从 context 取用户身份；所有接口兼容 `Authorization: Bearer` 头部
- **会话管理**：Token 超时自动过期，定期清理过期会话，每次访问刷新过期时间
- **速率限制**：登录/注册/发消息按 IP 限频（令牌桶算法），防止暴力破解和消息冲刷
- **数据库**：SQLite，使用参数化查询防 SQL 注入，事务保证好友关系一致性
- **日志**：使用 Go 标准库 `log/slog` 结构化日志，支持按环境切换输出格式
- **头像存储**：图片上传保存为文件，emoji 直接存入数据库，灵活兼容；限制 2MB 大小
- **WebSocket**：JSON 协议通信，先建连接再发送 `{type:"auth", token}` 认证，不暴露 token 在 URL；支持公聊、私聊、群聊；具备指数退避自动重连机制
- **TLS**：`-tls` 标志启动时自动生成 ECDSA 自签名证书，无需手动配置
- **优雅关闭**：捕获 SIGINT/SIGTERM，通过 `sync.WaitGroup` 等待所有 goroutine 退出，15 秒超时强制退出
- **水平扩展**：`SessionStore` 接口抽象会话存储，支持 Memory / Redis 双后端；`MessageBus` 接口通过 Redis Pub/Sub 实现跨服务器消息分发，消息携带 `OriginID` 去重，避免广播回环
- **离线消息**：私聊目标不在线时消息存入数据库，上线后可加载历史记录
- **群组校验**：群名不能为空且不超过 32 字符
- **前端 UI**：无框架纯 JavaScript，现代 CSS（玻璃拟态 + 渐变主题），支持深色模式

## 安全特性

- 密码使用 bcrypt 哈希存储
- 所有数据库查询使用参数化语句，防止 SQL 注入
- 用户输入经过 XSS 过滤和长度截断
- 文件上传路径经过安全净化，防止路径穿越
- 请求体限制 1MB，防止内存耗尽
- CORS 跨域配置白名单
- 会话 Token 使用 `crypto/rand` 加密随机数生成

## 测试

项目包含 70+ 个单元测试，覆盖 model、db、server 三层：

```bash
go test ./... -v -count=1 -timeout 60s
```

### 测试结构

| 包 | 文件 | 覆盖内容 |
|---|---|---|
| `internal/config` | `config_test.go` | 默认值、Duration JSON 解析、环境变量覆盖、配置文件加载 |
| `internal/model` | `model_test.go` | Session 过期边界测试、常量验证 |
| `internal/db` | `db_test.go` | 用户注册/认证、群组 CRUD、消息持久化、好友系统、并发读 |
| `internal/server` | `ratelimit_test.go` | 限流器正常/超额/窗口重置、并发安全 |
| `internal/server` | `server_test.go` | 输入安全过滤、会话生命周期、在线用户、改名、优雅关闭 |
| `internal/server` | `redis_session_test.go` | Redis 会话存储 CRUD、TTL 自动过期 |
| `internal/server` | `redis_bus_test.go` | Redis 消息总线发布/接收、自身消息过滤 |
| `internal/server` | `web_test.go` | HTTP handler 测试（登录/注册/发消息/好友/群组/历史等） |

### 测试特点

- **表格驱动测试**：认证、输入过滤等使用数据驱动用例
- **子测试命名**：所有用例通过 `t.Run()` 命名，可独立运行与定位
- **Mock 存储层**：HTTP 测试使用 `mockStorage` 实现 `Storage` 接口，无需真实数据库
- **内存 SQLite**：数据库测试使用 `:memory:` 模式，互相隔离无污染
- **并发安全验证**：限流器 goroutine 并发测试、数据库并发读验证
- **边界值覆盖**：用户名/密码长度边界、消息截断、Session 过期

## License

MIT

## 更新日志

### 2026-05-04

- **修复**：`BroadCastFromWeb()` 缺少 WebSocket 客户端消息推送，HTTP API 发送的公聊消息现在会正确转发给所有 WebSocket 客户端
- **修复**：Dockerfile 增加 `-env prod` 参数，确保生产环境加载正确配置
- **修复**：TLS 证书生成失败时返回错误而非静默降级为明文
- **修复**：速率限制器过期条目清理从"每 100 次触发"改为每次新键插入时清理，防止内存泄漏
- **修复**：TCP 与 WebSocket 群聊/私聊的锁模式简化，消除重复的 `RUnlock` 分支
- **修复**：TCP 客户端消息前缀统一，Web 端发消息不再带 `[WEB]` 前缀
- **清理**：移除废弃的 `sendWSMessage()` 函数、空测试函数、重复的 `ALTER TABLE` 迁移代码
- **优化**：`gofmt -w` 统一全项目代码格式
- **优化**：添加 `Storage.Close()` 接口和 `Database.Close()` 方法，服务关闭时正确释放数据库资源
- **新增**：`RedisSessionStore` — Redis 后端的 SessionStore 实现，支持水平扩展，TTL 由 Redis 自动管理
- **新增**：`RedisMessageBus` — Redis Pub/Sub 消息总线，实现跨服务器消息分发（公聊/私聊/群聊/系统通知）
- **新增**：`BusMessage.OriginID` 消息源标识，接收端自动过滤自身消息，防止广播回环
- **新增**：docker-compose.yml — 一键启动 Redis 7 + 应用服务，自动启用 Redis 会话存储和消息总线
- **新增**：`IM_SESSION_BACKEND`、`IM_REDIS_ADDR`、`IM_REDIS_PASSWORD`、`IM_REDIS_DB` 配置项
- **新增**：Dockerfile 安装 curl 用于 Redis 健康检查
- **测试**：`redis_session_test.go` — Redis 会话存储 CRUD + TTL 测试
- **测试**：`redis_bus_test.go` — 消息总线跨服务器收发 + 自身消息过滤测试
- **测试**：所有 Redis 测试自动检测 Redis 可用性，不可用时优雅跳过

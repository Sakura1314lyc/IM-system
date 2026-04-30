# IM-system

一个基于 Go 的轻量级即时通讯系统，支持 **TCP 命令行客户端** 和 **Web UI** 双端接入，后端集成 SQLite 持久化、REST API、SSE 实时推送。

## 项目结构

```
├── main.go                  # 启动入口（TCP + Web 双服务 + 优雅关闭）
├── config/                  # 多环境配置文件
│   ├── dev.json             #   开发环境（默认）
│   ├── prod.json            #   生产环境（TLS + 宽松限流）
│   └── test.json            #   测试环境（内存 DB + 短超时）
├── internal/
│   ├── config/config.go     # 配置管理（JSON + 环境变量覆盖）
│   ├── model/model.go       # 数据模型（User, Message, Group, Session 等）
│   ├── db/db.go             # SQLite 数据库层（参数化查询，事务保证）
│   └── server/
│       ├── server.go        # 服务端核心（TCP 监听、消息广播、会话管理）
│       ├── user.go          # 用户模型与 TCP 命令处理
│       ├── web.go           # REST API、SSE、中间件、TLS 证书
│       └── ratelimit.go     # 速率限制器
├── cmd/
│   ├── client/main.go       # TCP 命令行客户端
│   └── web/main.go          # Web 网关代理（可选独立部署）
└── web/                     # 前端 UI
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
- ✅ 会话 Token 24 小时自动过期 + 后台清理
- ✅ 登录/注册/消息发送速率限制（基于 IP，每分钟 10 次）
- ✅ 密码 bcrypt 哈希存储，密码哈希不暴露于 API 响应
- ✅ 服务端空闲超时踢出（10 分钟无消息）
- ✅ 输入安全过滤（防 XSS）
- ✅ 可选 TLS 加密（自动生成 ECDSA 自签名证书）
- ✅ 端口被占用时自动回退
- ✅ 优雅关闭（SIGINT/SIGTERM 信号处理）
- ✅ IPv6 兼容
- ✅ 多环境配置管理（dev/prod/test JSON + 环境变量覆盖）

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
# 指定运行环境（dev/prod/test），默认 dev
go run . -env prod

# 环境变量覆盖配置（IM_ 前缀）
IM_SERVER_PORT=9999 IM_SESSION_TTL=12h go run .

# 传统参数仍可用，会覆盖配置文件对应值
go run . -ip 127.0.0.1 -port 8888 -tls
```

> `-tls` 会自动生成 ECDSA P256 自签名证书用于开发环境，生产环境请使用正规证书。

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
| `IM_IDLE_TIMEOUT` | 用户超时踢出 | `10m` |
| `IM_RATE_LIMIT` | 每分钟最大请求数 | `30` |
| `IM_MAX_MSG_LENGTH` | 消息最大长度 | `2000` |

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

- **分层设计**：`model` → `db` → `server` 三层依赖，无循环引用
- **并发模型**：每个 TCP 连接一个 goroutine，消息通过 `channel` 分发
- **认证中间件**：GET 路由统一使用 `authQueryMiddleware`，从 context 取用户身份
- **会话管理**：Token 24 小时过期，30 分钟轮询清理过期会话
- **速率限制**：登录/注册/发消息按 IP 限频，防止暴力破解和消息冲刷
- **数据库**：SQLite，使用参数化查询防 SQL 注入，事务保证好友关系一致性
- **TLS**：`-tls` 标志启动时自动生成 ECDSA 自签名证书，无需手动配置
- **优雅关闭**：捕获 SIGINT/SIGTERM，关闭 listener 释放端口

## 测试

项目包含 45+ 个单元测试，覆盖 model、db、server 三层，使用 `go test` 直接运行：

```bash
go test ./... -v -count=1 -timeout 60s
```

### 测试结构

| 包 | 文件 | 覆盖内容 |
|---|---|---|
| `internal/config` | `config_test.go` | 默认值、Duration JSON 解析、环境变量覆盖、配置文件加载 |
| `internal/model` | `model_test.go` | Session 过期边界测试、常量验证 |
| `internal/db` | `db_test.go` | 用户注册/认证、群组 CRUD、消息持久化（公聊/私聊/群聊）、好友系统（双向事务）、个人资料更新、边界值、并发读 |
| `internal/server` | `ratelimit_test.go` | 限流器正常/超额/窗口重置、不同 key 隔离、并发安全、零限流 |
| `internal/server` | `server_test.go` | 输入安全过滤（XSS/截断）、会话 Token 全生命周期、在线用户查询、改名、优雅关闭 |

### 测试特点

- **表格驱动测试**：认证、输入过滤等使用数据驱动用例
- **子测试命名**：所有用例通过 `t.Run()` 命名，可独立运行与定位
- **内存 SQLite**：数据库测试使用 `:memory:` 模式，互相隔离无污染
- **并发安全验证**：限流器 goroutine 并发测试、数据库并发读验证
- **边界值覆盖**：用户名 3/20 字符边界、密码 6 字符边界、Session 纳秒级过期

## License

MIT

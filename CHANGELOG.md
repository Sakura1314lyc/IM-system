# IM-system 更新日志

## ✨ 最新更新

### 安全增强提升

- 🔐 **Web 登录令牌认证**：配置 token-based 认证，`/api/login` 返回加密 token，所有 Web API 调用需提供 token。
- 📝 **安全会话管理**：添加 `SessionTokens` 映射，支持独立的每用户认证状态。

### 功能完善

- 👤 **头像系统**：
  - 用户登录/注册时可上传自定义头像图片（支持 JPG/PNG/GIF）。
  - 图片自动转换为 base64 编码存储到数据库。
  - 支持 `/api/avatar` POST 端点动态更新头像。
  - 头像信息同步至 TCP 客户端和 Web 端展示。
  - 前端所有位置正确显示上传的图片头像或 emoji 头像。

- 👥 **完整群组管理**：
  - 创建/加入/离开群组 API (`/api/group`)。
  - 获取用户群组列表 (`/api/groups`)。
  - 群消息广播与保存到数据库。

- 📜 **消息历史记录**：
  - `/api/history` 端点支持加载公聊/私聊/群聊历史消息。
  - 消息分页加载（默认 50 条）。
  - 前端"加载历史"按钮即时展示历史消息。

### UI/UX 改进

- ✨ **二次元主题强化**：
  - 粉紫色渐变背景与玻璃态卡片设计。
  - 头像选择器支持悬停放大、旋转、发光特效。
  - 消息动画（滑入、淡出）提升沉浸感。
  - 暗黑模式完整支持。

- 📱 **响应式设计**：
  - 侧边栏和消息区域在移动端自适应布局。
  - 头像选择器与设置面板优化。

- 🎨 **设置面板扩充**：
  - 字体大小调节。
  - 深色模式切换。
  - 消息声音/浏览器通知选项。

### 数据库增强

- 新增 `DBMessageExt` 类型支持消息关联用户/群组名查询。
- 消息表支持 `public`/`private`/`group` 三类消息类型。
- 群组成员表与用户群组查询优化。

### 文档更新

- 🔄 README 文档完整更新，包含：
  - token 认证说明
  - 头像功能描述
  - 历史消息加载说明
  - Web API 完整端点列表
  - 使用示例流程

## 技术细节

### 后端新端点

```
POST   /api/login         认证用户并返回 token
GET    /api/events        SSE 获取消息流（需 token）
GET    /api/online        获取在线用户列表
GET    /api/groups        获取用户群组（需 token）
POST   /api/group         创建/加入/离开群组（需 token）
POST   /api/avatar        更新头像（需 token）
GET    /api/history       加载消息历史（需 token）
POST   /api/send          发送消息（需 token）
```

### 前端新功能

- 登录/注册时选择头像
- 消息加载与历史查询按钮
- 设置面板头像更新
- 群组加入/离开流程

### 数据库查询

新增函数：
- `db.go`: `UpdateUserAvatar()`、`GetUserGroups()`、`GetPublicMessages()`、`GetGroupMessages()`、`GetPrivateMessages()`
- `server.go`: `handleHistory()` 端点


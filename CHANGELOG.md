# 变更日志

## 2026-03-06

### 新增功能

#### GitHub OAuth 认证

- 添加了 GitHub OAuth 认证流程
- 支持从环境变量读取配置：
  - `GITHUB_CLIENT_ID` - GitHub OAuth 客户端 ID
  - `GITHUB_CLIENT_SECRET` - GitHub OAuth 客户端密钥
  - `ALLOWED_USER_IDS` - 允许登录的用户 ID 列表（逗号分隔）
- 添加了认证路由：
  - `GET /auth/github` - 启动 GitHub OAuth 流程
  - `GET /auth/github/callback` - GitHub OAuth 回调处理
  - `GET /auth/logout` - 用户登出
  - `GET /auth/me` - 获取当前用户信息
- 添加了认证中间件 `AuthMiddleware()`
- WebSocket 路由现在需要认证

#### 前端登录页面

- 添加了登录页面组件 (`web/src/components/Login.tsx`)
- 显示 "Sign in with GitHub" 按钮
- 检查认证状态
- 支持重定向到 GitHub OAuth 流程

### 文件变更

#### 新增文件

- `auth.go` - GitHub OAuth 认证实现
- `web/src/components/Login.tsx` - 登录页面组件
- `AUTH_SETUP.md` - 认证设置指南
- `IMPLEMENTATION_SUMMARY.md` - 实现总结
- `CHANGELOG.md` - 变更日志
- `cmd/red-pill-shell/main.go` - red-pill-shell 命令

#### 修改文件

- `main.go` - 集成认证功能
- `web/src/App.tsx` - 添加认证状态管理
- `readme.md` - 更新文档
- `go.mod` - 添加 OAuth2 依赖
- `go.sum` - 添加 OAuth2 依赖
- `Dockerfile` - 添加 red-pill-shell 构建
- `makefile` - 添加 red-pill-shell 构建目标
- `web/pnpm-lock.yaml` - 依赖更新
- `.gitignore` - 忽略构建产物

### 使用方法

1. 设置环境变量：
   ```bash
   export GITHUB_CLIENT_ID=your_client_id
   export GITHUB_CLIENT_SECRET=your_client_secret
   export ALLOWED_USER_IDS=12345678,87654321  # 可选
   ```

2. 运行应用：
   ```bash
   ./wsterm-new -bind :8080
   ```

3. 访问应用：
   - 打开浏览器访问 `http://localhost:8080/web`
   - 如果未登录，会显示登录页面
   - 点击 "Sign in with GitHub" 按钮
   - 授权后即可访问终端

### 安全特性

1. **用户限制**: 只允许指定的 GitHub 用户 ID 登录
2. **Cookie 安全**: 使用 HttpOnly 和 Secure 标志
3. **状态验证**: OAuth 流程中验证状态参数
4. **会话管理**: 使用 cookie 存储认证信息

### 注意事项

1. 需要先在 GitHub 创建 OAuth App
2. 回调 URL 需要配置为 `http://your-domain.com/auth/github/callback`
3. 生产环境中建议启用 SSL (`ENABLE_SSL=true`)
4. 不要将 `GITHUB_CLIENT_SECRET` 提交到版本控制系统

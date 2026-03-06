# GitHub OAuth 认证设置指南

## 概述

本应用已添加 GitHub OAuth 认证功能，只允许特定用户 ID 的用户登录。

## 环境变量配置

需要设置以下环境变量：

### 必需的环境变量

1. **GITHUB_CLIENT_ID**
   - GitHub OAuth App 的客户端 ID
   - 示例：`GITHUB_CLIENT_ID=your_client_id_here`

2. **GITHUB_CLIENT_SECRET**
   - GitHub OAuth App 的客户端密钥
   - 示例：`GITHUB_CLIENT_SECRET=your_client_secret_here`

### 可选的环境变量

3. **ALLOWED_USER_IDS**
   - 允许登录的 GitHub 用户 ID 列表（逗号分隔）
   - 如果不设置，所有用户都可以登录
   - 示例：`ALLOWED_USER_IDS=12345678,87654321`

4. **OAUTH_REDIRECT_URL**
   - OAuth 回调地址
   - 默认会根据协议和绑定地址自动生成
   - 示例：`OAUTH_REDIRECT_URL=https://your-domain.com/auth/github/callback`

5. **ENABLE_SSL**
   - 是否启用 SSL
   - 示例：`ENABLE_SSL=true`

## 创建 GitHub OAuth App

1. 访问 GitHub Settings > Developer settings > OAuth Apps
2. 点击 "New OAuth App"
3. 填写应用信息：
   - Application name: Terminal App
   - Homepage URL: `http://your-domain.com` (或 `https://your-domain.com`)
   - Authorization callback URL: `http://your-domain.com/auth/github/callback` (或 `https://...`)
4. 点击 "Register application"
5. 记录 Client ID 和 Client Secret

## 运行应用

### 使用环境变量运行

```bash
export GITHUB_CLIENT_ID=your_client_id
export GITHUB_CLIENT_SECRET=your_client_secret
export ALLOWED_USER_IDS=12345678,87654321

./wsterm-new -bind :8080
```

### 使用 Docker 运行

```bash
docker run -d \
  -e GITHUB_CLIENT_ID=your_client_id \
  -e GITHUB_CLIENT_SECRET=your_client_secret \
  -e ALLOWED_USER_IDS=12345678,87654321 \
  -p 8080:8080 \
  wsterm
```

## 认证流程

1. 用户访问应用首页
2. 如果未登录，显示登录页面
3. 用户点击 "Sign in with GitHub"
4. 重定向到 GitHub 授权页面
5. 用户授权后，重定向回应用
6. 应用验证用户 ID 是否在允许列表中
7. 如果验证通过，设置 cookie 并允许访问终端

## API 端点

- `GET /auth/github` - 启动 GitHub OAuth 流程
- `GET /auth/github/callback` - GitHub OAuth 回调
- `GET /auth/logout` - 登出
- `GET /auth/me` - 获取当前用户信息

## 故障排除

### "GitHub auth not configured" 错误

确保已设置 `GITHUB_CLIENT_ID` 和 `GITHUB_CLIENT_SECRET` 环境变量。

### "User not allowed" 错误

确保您的 GitHub 用户 ID 在 `ALLOWED_USER_IDS` 列表中，或者移除该限制（不设置 `ALLOWED_USER_IDS`）。

### 如何获取 GitHub 用户 ID

1. 访问 https://api.github.com/users/your_username
2. 查看 `id` 字段的值

## 安全注意事项

1. 不要将 `GITHUB_CLIENT_SECRET` 提交到版本控制系统
2. 使用 HTTPS 协议进行 OAuth 回调
3. 定期轮换 Client Secret
4. 限制允许的用户 ID 列表

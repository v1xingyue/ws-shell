# ws-shell

[![Deploy with Vercel](https://vercel.com/button)](https://vercel.com/new/clone?repository-url=https://github.com/v1xingyue/ws-shell)

[English](readme.md) | 中文

基于 WebSocket 的网页终端，支持可选的用户名密码或 GitHub OAuth 认证。登录态有效期为两周。

## 功能

- 通过 WebSocket + PTY 在浏览器里运行真实交互式 shell。
- 终端入口是 `/console`，其余路径会转发到 `BACKGROUND_SERVER_URL`，默认是 `http://localhost:3000`。
- 支持键盘输入、终端输出和浏览器侧窗口大小同步。
- 可用 `-fork` 指定 shell，例如 `/bin/bash`、`/bin/zsh` 或 `/bin/sh`。
- 默认单用户模式；开启多用户模式后可用 `?user=username` 切换系统用户。
- 支持可选用户名密码或 GitHub OAuth 访问保护；两种方式都会写入两周有效的签名登录态。
- 支持用 `ALLOWED_USER_IDS` 按 GitHub 数字用户 ID 限制登录。
- 支持从环境变量或根目录 `.env` 加载配置。
- 本地可用内置 HTTPS；放在平台 HTTPS 代理后也可关闭应用内 TLS。
- 支持构建并运行成小体积 Docker 镜像。

## Vercel Dockerfile 部署能做什么

部署按钮会让 Vercel 指向本仓库。`Dockerfile.vercel` 会构建 Vite 前端、构建 Go 服务，并启动：

```bash
/app/bin/wsterm -bind :80 -fork /bin/sh
```

部署后可以得到：

- 一个浏览器终端，地址是 `https://你的部署域名/console`。
- 基于 WebSocket 的终端会话，实际运行在部署出来的 Alpine 容器里的 `/bin/sh`。
- 公网地址由平台提供 HTTPS，应用内部关闭 SSL，因为 TLS 由平台代理处理。
- 可通过 Vercel 环境变量开启认证：
  - `AUTH_USERNAME`
  - `AUTH_PASSWORD`
  - `GITHUB_CLIENT_ID`
  - `GITHUB_CLIENT_SECRET`
  - `ALLOWED_USER_IDS`
  - `OAUTH_REDIRECT_URL`
- 一个临时容器 shell，可用于查看部署环境、测试网络访问、检查环境变量、运行镜像里已有的工具。

需要注意：

- shell 运行在部署容器里，不是在你的本地电脑上。
- 文件和进程不会在重新部署、重启或容器替换后持久保存。
- 只能使用镜像里已有的工具；需要更多工具时，在 `Dockerfile.vercel` 里安装。
- 不要在没有 OAuth 和白名单的情况下公开暴露。

## 构建

```bash
cd web && pnpm install && pnpm run build
go build
```

## 使用方法

### 1. 无授权 HTTP（默认）

```bash
./wsterm
```

默认监听 `:8080`，浏览器访问 `http://服务器IP:8080/console`。

### 2. 开启 HTTPS

```bash
./wsterm -enable-ssl
```

或通过项目根目录 `.env` 中的环境变量：

```bash
export ENABLE_SSL=true
./wsterm
```

首次运行会生成 `cert.pem` 和 `key.pem`。

### 3. 指定 Shell

```bash
./wsterm -fork=/bin/bash
# 或
./wsterm -fork=/bin/zsh
```

### 4. 用户名密码认证

同时设置用户名和密码即可启用：

```bash
export AUTH_USERNAME=admin
export AUTH_PASSWORD=change-me

./wsterm
```

登录成功后，浏览器里的签名登录态有效期为 14 天。

### 5. GitHub 授权

设置环境变量，或写入项目根目录 `.env` 后启动：

```bash
export GITHUB_CLIENT_ID=your_github_client_id
export GITHUB_CLIENT_SECRET=your_github_client_secret
export ALLOWED_USER_IDS=12345678,87654321  # 可选：仅允许这些用户 ID

./wsterm
```

GitHub 登录成功后，浏览器里的签名登录态有效期为 14 天。

组合示例：

```bash
export GITHUB_CLIENT_ID=xxx
export GITHUB_CLIENT_SECRET=xxx
export ALLOWED_USER_IDS=12345678
./wsterm -enable-ssl -fork=/bin/bash
```

## 环境变量

程序会从**当前工作目录**的 `.env` 加载环境变量（可选）。

配置方法：

- Shell：启动前执行 `export AUTH_USERNAME=admin`。
- `.env`：在项目根目录写入同样的 `KEY=value`。
- Docker：用 `-e KEY=value` 传入。
- Vercel：在 Project Settings -> Environment Variables 添加同名变量。
- `vercel-vm-factory`：使用 `--auth-mode basic|github|both|none` 以及对应认证参数。

| 变量 | 说明 |
|---|---|
| `AUTH_USERNAME` | 用户名密码登录的用户名，与密码同时设置则启用认证 |
| `AUTH_PASSWORD` | 用户名密码登录的密码 |
| `AUTH_SESSION_SECRET` | 可选的 cookie 签名密钥；默认使用已配置的认证密钥 |
| `GITHUB_CLIENT_ID` | GitHub OAuth 客户端 ID，与 SECRET 同时设置则启用 GitHub 认证 |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth 客户端密钥 |
| `ALLOWED_USER_IDS` | 允许登录的 GitHub 用户 ID，逗号分隔；不设则允许所有已登录 GitHub 用户 |
| `OAUTH_REDIRECT_URL` | OAuth 回调地址；不设则按请求 Host 自动拼 |
| `OAUTH_LOCAL_DEV` | 设为 `true` 时，本地调试通过公网回调接收 GitHub code，再转回本地 |
| `OAUTH_LOCAL_REDIRECT_URL` | 本地调试使用的公网 OAuth 回调，默认 `https://ws-shell.vercel.app/auth/github/callback?local=true` |
| `OAUTH_LOCAL_CALLBACK_URL` | 公网回调转回的本地地址，默认 `http://localhost/auth/github/callback` |
| `ENABLE_SSL` | `true` 时启用 HTTPS |
| `-bind` | 监听地址，默认 `:8080` |
| `-fork` | 启动的 shell，默认 `/bin/bash` |

## GitHub OAuth 配置

### 创建 OAuth App

1. 打开 [GitHub Settings -> Developer settings -> OAuth Apps](https://github.com/settings/developers)。
2. 点击 "New OAuth App"。
3. 填写：
   - **Application name**：任意名称，如 Terminal
   - **Homepage URL**：如 `http://你的域名:8080` 或 `https://你的域名`
   - **Authorization callback URL**：必须与应用实际回调地址一致，例如：
     - 本地：`http://localhost:8080/auth/github/callback`
     - 内网：`http://192.168.3.51:8080/auth/github/callback`
     - 公网：`https://你的域名/auth/github/callback`
4. 保存后记录 **Client ID**，并生成 **Client Secret**。

### 回调地址说明

- 程序默认按**当前请求的 Host** 自动拼回调地址。
- GitHub 里填的 **Authorization callback URL** 必须和实际访问地址一致。例如用 `http://192.168.3.51:8080/console` 访问，就填 `http://192.168.3.51:8080/auth/github/callback`。
- 仅在反向代理等导致 Host 与真实访问地址不一致时，才需要设置 `OAUTH_REDIRECT_URL`。
- 如果线上 OAuth 回调已固定，本地调试可设置 `OAUTH_LOCAL_DEV=true`；公网回调会把一次性的 GitHub code 转回 `OAUTH_LOCAL_CALLBACK_URL`。

### 获取 GitHub 用户 ID

白名单使用**用户 ID**（数字），不是登录名。

推荐方式：浏览器打开 `https://api.github.com/users/你的登录名`，在返回 JSON 中找到 `"id"`。

命令行：

```bash
curl -s https://api.github.com/users/你的登录名 | grep '"id"'
```

登录 GitHub 后也可以访问 `https://api.github.com/user`，查看返回中的 `id` 字段。

配置示例：`ALLOWED_USER_IDS=974169` 或 `ALLOWED_USER_IDS=12345,67890`。

## 认证流程

1. 用户访问应用首页。
2. 未登录则显示登录页。
3. 使用用户名密码登录，或在配置 GitHub 后点击 "Sign in with GitHub"。
4. 通过后写入签名 cookie，并允许访问终端。

未设置任何认证环境变量时，以无认证模式运行，所有人可直接访问 `/console`。

## API

- `POST /auth/password` - 用户名密码登录
- `GET /auth/github` - 发起 GitHub OAuth
- `GET /auth/github/callback` - OAuth 回调
- `GET /auth/logout` - 登出
- `GET /auth/me` - 当前用户信息
- `POST /console/:name/mcp` - 最小 MCP Streamable HTTP 入口，提供 `shell` 工具

通过 `BACKGROUND_SERVER_URL` 配置真实 Web 端转发目标。
通过 `MCP_TOKEN` 启用 MCP；客户端可用 `Authorization: Bearer TOKEN` 或 `/console/vm/mcp?token=TOKEN`。

## 故障排除

**"GitHub auth not configured"**

未设置 `GITHUB_CLIENT_ID` 或 `GITHUB_CLIENT_SECRET`。不需要认证时可直接访问 `/console`。

**redirect_uri 报错 / 跳转不对**

GitHub 里填的 **Authorization callback URL** 与程序使用的回调地址不一致。确保域名、端口、协议一致；必要时设置 `OAUTH_REDIRECT_URL`。

**"User not allowed"**

当前 GitHub 用户 ID 不在 `ALLOWED_USER_IDS` 中。将你的用户 ID 加入白名单，或暂时不设置 `ALLOWED_USER_IDS` 以允许所有已登录用户。

## 安全注意

- 不要将 `AUTH_PASSWORD` 或 `GITHUB_CLIENT_SECRET` 提交到仓库。
- 生产环境建议用 HTTPS 做 OAuth 回调。
- 定期轮换 Client Secret。
- 用 `ALLOWED_USER_IDS` 限制可登录用户。

## Docker

使用已发布镜像：

```bash
docker run -d --name ws-shell -p 8090:8080 ghcr.io/v1xingyue/ws-shell:main
```

访问 `http://0.0.0.0:8090/console`。启用认证时传入环境变量：

```bash
docker run -d --name ws-shell \
  -e AUTH_USERNAME=admin \
  -e AUTH_PASSWORD=change-me \
  -e GITHUB_CLIENT_ID=xxx \
  -e GITHUB_CLIENT_SECRET=xxx \
  -e ALLOWED_USER_IDS=12345678 \
  -p 8090:8080 \
  ghcr.io/v1xingyue/ws-shell:main
```

### red-pill-shell

```bash
docker run -d --name ws-shell -e red-pill-token=true -p 8090:8080 ghcr.io/v1xingyue/ws-shell:main
```

## 其他

- 建议以 **root** 运行。多用户模式下需要切换进程 UID/GID。
- 命令行参数包括 `-bind`、`-enable-ssl`、`-fork`、`-debug`、`-single` 等，可通过 `./wsterm -h` 查看。

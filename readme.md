# ws-shell

[![Deploy with Vercel](https://vercel.com/button)](https://vercel.com/new/clone?repository-url=https://github.com/v1xingyue/ws-shell)

English | [中文](#中文)

WebSocket-based web terminal with optional GitHub OAuth authentication.

## Features

- WebSocket terminal
- Optional GitHub OAuth login, enabled after credentials are configured
- GitHub user ID allowlist
- HTTPS support through generated certificates or environment variables
- `.env` environment variable loading

## Build

```bash
cd web && pnpm install && pnpm run build
go build
```

## Usage

### 1. Unauthenticated HTTP (default)

```bash
./wsterm
```

The server listens on `:8080` by default. Open `http://SERVER_IP:8080/web`.

### 2. Enable HTTPS

```bash
./wsterm -enable-ssl
```

Or use an environment variable from the project root `.env`:

```bash
export ENABLE_SSL=true
./wsterm
```

On first run, `cert.pem` and `key.pem` are generated.

### 3. Choose a Shell

```bash
./wsterm -fork=/bin/bash
# or
./wsterm -fork=/bin/zsh
```

### 4. GitHub Authentication

Set environment variables, or put them in the project root `.env`, then start the server:

```bash
export GITHUB_CLIENT_ID=your_github_client_id
export GITHUB_CLIENT_SECRET=your_github_client_secret
export ALLOWED_USER_IDS=12345678,87654321  # optional: only allow these user IDs

./wsterm
```

Combined example:

```bash
export GITHUB_CLIENT_ID=xxx
export GITHUB_CLIENT_SECRET=xxx
export ALLOWED_USER_IDS=12345678
./wsterm -enable-ssl -fork=/bin/bash
```

## Environment Variables

The program optionally loads environment variables from `.env` in the current working directory.

| Variable | Description |
|---|---|
| `GITHUB_CLIENT_ID` | GitHub OAuth client ID. Authentication is enabled when both ID and secret are set. |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth client secret |
| `ALLOWED_USER_IDS` | Comma-separated GitHub user IDs allowed to log in. If unset, any logged-in GitHub user is allowed. |
| `OAUTH_REDIRECT_URL` | OAuth callback URL. If unset, it is built from the request host. |
| `ENABLE_SSL` | Enables HTTPS when set to `true` |
| `-bind` | Listen address, default `:8080` |
| `-fork` | Shell to start, default `/bin/bash` |

## GitHub OAuth Setup

### Create an OAuth App

1. Open [GitHub Settings -> Developer settings -> OAuth Apps](https://github.com/settings/developers).
2. Click "New OAuth App".
3. Fill in:
   - **Application name**: any name, such as Terminal
   - **Homepage URL**: for example `http://your-domain:8080` or `https://your-domain`
   - **Authorization callback URL**: must match the callback URL used by this app, for example:
     - Local: `http://localhost:8080/auth/github/callback`
     - LAN: `http://192.168.3.51:8080/auth/github/callback`
     - Public: `https://your-domain/auth/github/callback`
4. Save the app, copy the **Client ID**, and generate a **Client Secret**.

### Callback URL

- The program builds the callback URL from the current request host by default.
- The **Authorization callback URL** in GitHub must match the real access URL. For example, if you open `http://192.168.3.51:8080/web`, use `http://192.168.3.51:8080/auth/github/callback`.
- Set `OAUTH_REDIRECT_URL` only when a reverse proxy or similar setup makes the request host differ from the public URL.

### Get a GitHub User ID

The allowlist uses numeric GitHub user IDs, not usernames.

Recommended: open `https://api.github.com/users/YOUR_LOGIN` in a browser and read the `"id"` field.

Command line:

```bash
curl -s https://api.github.com/users/YOUR_LOGIN | grep '"id"'
```

If logged in to GitHub, you can also open `https://api.github.com/user` and read the `id` field.

Examples: `ALLOWED_USER_IDS=974169` or `ALLOWED_USER_IDS=12345,67890`.

## Authentication Flow

1. User opens the app.
2. If not logged in, the login page is shown.
3. Clicking "Sign in with GitHub" redirects to GitHub.
4. GitHub redirects back to the app after authorization.
5. The app checks whether the user ID is in `ALLOWED_USER_IDS`.
6. If allowed, a cookie is written and the terminal becomes available.

If `GITHUB_CLIENT_ID` or `GITHUB_CLIENT_SECRET` is not set, the app runs without authentication and `/web` is directly accessible.

## API

- `GET /auth/github` - start GitHub OAuth
- `GET /auth/github/callback` - OAuth callback
- `GET /auth/logout` - log out
- `GET /auth/me` - current user info

## Troubleshooting

**"GitHub auth not configured"**

`GITHUB_CLIENT_ID` or `GITHUB_CLIENT_SECRET` is missing. If authentication is not needed, open `/web` directly.

**redirect_uri error or wrong redirect**

The **Authorization callback URL** in GitHub does not match the callback URL used by the program. Make sure the domain, port, and protocol match. Set `OAUTH_REDIRECT_URL` if needed.

**"User not allowed"**

The current GitHub user ID is not in `ALLOWED_USER_IDS`. Add your user ID to the allowlist, or leave `ALLOWED_USER_IDS` unset to allow all logged-in GitHub users.

## Security Notes

- Do not commit `GITHUB_CLIENT_SECRET`.
- Use HTTPS for OAuth callbacks in production.
- Rotate the Client Secret regularly.
- Use `ALLOWED_USER_IDS` to restrict access.

## Docker

```bash
docker run -d --name ws-shell -p 8090:8080 ghcr.io/v1xingyue/ws-shell:main
```

Open `http://0.0.0.0:8090/web`. Pass environment variables to enable GitHub authentication:

```bash
docker run -d --name ws-shell \
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

## Notes

- Running as **root** is recommended. Multi-user mode needs process UID/GID switching.
- CLI flags include `-bind`, `-enable-ssl`, `-fork`, `-debug`, and `-single`. Run `./wsterm -h` to see all options.

---

## 中文

基于 WebSocket 的网页终端，支持可选的 GitHub OAuth 认证。

## 功能

- WebSocket 终端
- 可选 GitHub OAuth 登录，配置凭据后启用
- 按 GitHub 用户 ID 白名单限制访问
- 支持 HTTPS，使用生成证书或环境变量
- 支持 `.env` 环境变量加载

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

默认监听 `:8080`，浏览器访问 `http://服务器IP:8080/web`。

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

### 4. GitHub 授权

设置环境变量，或写入项目根目录 `.env` 后启动：

```bash
export GITHUB_CLIENT_ID=your_github_client_id
export GITHUB_CLIENT_SECRET=your_github_client_secret
export ALLOWED_USER_IDS=12345678,87654321  # 可选：仅允许这些用户 ID

./wsterm
```

组合示例：

```bash
export GITHUB_CLIENT_ID=xxx
export GITHUB_CLIENT_SECRET=xxx
export ALLOWED_USER_IDS=12345678
./wsterm -enable-ssl -fork=/bin/bash
```

## 环境变量

程序会从**当前工作目录**的 `.env` 加载环境变量（可选）。

| 变量 | 说明 |
|---|---|
| `GITHUB_CLIENT_ID` | GitHub OAuth 客户端 ID，与 SECRET 同时设置则启用认证 |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth 客户端密钥 |
| `ALLOWED_USER_IDS` | 允许登录的 GitHub 用户 ID，逗号分隔；不设则允许所有已登录 GitHub 用户 |
| `OAUTH_REDIRECT_URL` | OAuth 回调地址；不设则按请求 Host 自动拼 |
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
- GitHub 里填的 **Authorization callback URL** 必须和实际访问地址一致。例如用 `http://192.168.3.51:8080/web` 访问，就填 `http://192.168.3.51:8080/auth/github/callback`。
- 仅在反向代理等导致 Host 与真实访问地址不一致时，才需要设置 `OAUTH_REDIRECT_URL`。

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
3. 点击 "Sign in with GitHub" 跳转 GitHub 授权。
4. 授权后跳回应用。
5. 应用校验用户 ID 是否在 `ALLOWED_USER_IDS` 中。
6. 通过后写入 cookie，并允许访问终端。

未设置 `GITHUB_CLIENT_ID` 或 `GITHUB_CLIENT_SECRET` 时，以无认证模式运行，所有人可直接访问 `/web`。

## API

- `GET /auth/github` - 发起 GitHub OAuth
- `GET /auth/github/callback` - OAuth 回调
- `GET /auth/logout` - 登出
- `GET /auth/me` - 当前用户信息

## 故障排除

**"GitHub auth not configured"**

未设置 `GITHUB_CLIENT_ID` 或 `GITHUB_CLIENT_SECRET`。不需要认证时可直接访问 `/web`。

**redirect_uri 报错 / 跳转不对**

GitHub 里填的 **Authorization callback URL** 与程序使用的回调地址不一致。确保域名、端口、协议一致；必要时设置 `OAUTH_REDIRECT_URL`。

**"User not allowed"**

当前 GitHub 用户 ID 不在 `ALLOWED_USER_IDS` 中。将你的用户 ID 加入白名单，或暂时不设置 `ALLOWED_USER_IDS` 以允许所有已登录用户。

## 安全注意

- 不要将 `GITHUB_CLIENT_SECRET` 提交到仓库。
- 生产环境建议用 HTTPS 做 OAuth 回调。
- 定期轮换 Client Secret。
- 用 `ALLOWED_USER_IDS` 限制可登录用户。

## Docker

```bash
docker run -d --name ws-shell -p 8090:8080 ghcr.io/v1xingyue/ws-shell:main
```

访问 `http://0.0.0.0:8090/web`。启用 GitHub 认证时传入环境变量：

```bash
docker run -d --name ws-shell \
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

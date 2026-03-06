# ws-shell

基于 WebSocket 的网页终端，支持可选的 GitHub OAuth 认证。

## 功能

- WebSocket 终端
- 可选 GitHub OAuth 登录（配置凭据后启用）
- 按 GitHub 用户 ID 白名单限制访问
- 支持 HTTPS（内置证书或环境变量）
- 支持 `.env` 环境变量加载

---

## 构建

```bash
cd web && pnpm install && pnpm run build
go build
```

---

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

或通过环境变量（支持根目录 `.env`）：

```bash
export ENABLE_SSL=true
./wsterm
```

首次运行会生成 `cert.pem` / `key.pem`。

### 3. 指定 Shell

```bash
./wsterm -fork=/bin/bash
# 或
./wsterm -fork=/bin/zsh
```

### 4. GitHub 授权

设置环境变量（或写入项目根目录 `.env`）后启动：

```bash
export GITHUB_CLIENT_ID=your_github_client_id
export GITHUB_CLIENT_SECRET=your_github_client_secret
export ALLOWED_USER_IDS=12345678,87654321  # 可选：仅允许这些用户 ID

./wsterm
```

### 组合示例

```bash
export GITHUB_CLIENT_ID=xxx
export GITHUB_CLIENT_SECRET=xxx
export ALLOWED_USER_IDS=12345678
./wsterm -enable-ssl -fork=/bin/bash
```

---

## 环境变量

程序会从**当前工作目录**的 `.env` 加载环境变量（可选）。

| 变量 | 说明 |
|------|------|
| `GITHUB_CLIENT_ID` | GitHub OAuth 客户端 ID（与 SECRET 同时设置则启用认证） |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth 客户端密钥 |
| `ALLOWED_USER_IDS` | 允许登录的 GitHub 用户 ID，逗号分隔；不设则允许所有已登录 GitHub 用户 |
| `OAUTH_REDIRECT_URL` | OAuth 回调地址；不设则按请求 Host 自动拼 |
| `ENABLE_SSL` | `true` 时启用 HTTPS |
| `-bind` | 监听地址，默认 `:8080` |
| `-fork` | 启动的 shell，默认 `/bin/bash` |

---

## GitHub OAuth 配置

### 创建 OAuth App

1. 打开 [GitHub Settings → Developer settings → OAuth Apps](https://github.com/settings/developers)
2. 点击 "New OAuth App"
3. 填写：
   - **Application name**：任意，如 Terminal
   - **Homepage URL**：如 `http://你的域名:8080` 或 `https://你的域名`
   - **Authorization callback URL**：**必须**与下面「回调地址」一致，例如：
     - 本地：`http://localhost:8080/auth/github/callback`
     - 内网：`http://192.168.3.51:8080/auth/github/callback`
     - 公网：`https://你的域名/auth/github/callback`
4. 保存后记录 **Client ID**，并生成 **Client Secret**

### 回调地址说明

- 程序会按**当前请求的 Host** 自动拼回调地址（无需在 `.env` 写死）。
- 在 GitHub 里填的 **Authorization callback URL** 只要和实际访问地址一致即可，例如用 `http://192.168.3.51:8080/web` 访问，就填 `http://192.168.3.51:8080/auth/github/callback`。
- 仅在反向代理等导致 Host 与真实访问地址不一致时，才需设置 `OAUTH_REDIRECT_URL`。

### 获取 GitHub 用户 ID

白名单使用**用户 ID**（数字），不是登录名。获取方式任选其一：

**方法一（推荐）**：浏览器打开  
`https://api.github.com/users/你的登录名`  
在返回的 JSON 里找到 `"id"`，即用户 ID。

**方法二**：命令行  
`curl -s https://api.github.com/users/你的登录名 | grep '"id"'`

**方法三**：登录 GitHub 后访问  
`https://api.github.com/user`  
查看返回中的 `id` 字段。

配置示例：`ALLOWED_USER_IDS=974169` 或 `ALLOWED_USER_IDS=12345,67890`。

---

## 认证流程（启用认证时）

1. 用户访问应用首页
2. 未登录则显示登录页
3. 点击 "Sign in with GitHub" → 跳转 GitHub 授权
4. 授权后跳回应用
5. 校验用户 ID 是否在 `ALLOWED_USER_IDS` 中
6. 通过则写 cookie，可访问终端

未设置 `GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET` 时，以无认证模式运行，所有人可直接访问终端。

---

## API

- `GET /auth/github` — 发起 GitHub OAuth
- `GET /auth/github/callback` — OAuth 回调
- `GET /auth/logout` — 登出
- `GET /auth/me` — 当前用户信息

---

## 故障排除

**"GitHub auth not configured"**  
未设置 `GITHUB_CLIENT_ID` 或 `GITHUB_CLIENT_SECRET`。不需要认证时可直接访问 `/web`。

**redirect_uri 报错 / 跳转不对**  
GitHub 里填的 **Authorization callback URL** 与程序使用的回调地址不一致。确保与访问用的域名、端口、协议一致；必要时设置 `OAUTH_REDIRECT_URL`。

**"User not allowed"**  
当前 GitHub 用户 ID 不在 `ALLOWED_USER_IDS` 中。将你的用户 ID 加入白名单，或暂时不设置 `ALLOWED_USER_IDS` 以允许所有已登录用户。用户 ID 获取见上文。

---

## 安全注意

- 不要将 `GITHUB_CLIENT_SECRET` 提交到仓库
- 生产环境建议用 HTTPS 做 OAuth 回调
- 定期轮换 Client Secret
- 用 `ALLOWED_USER_IDS` 限制可登录用户

---

## Docker

```bash
docker run -d --name ws-shell -p 8090:8080 ghcr.io/v1xingyue/ws-shell:main
```

访问 `http://0.0.0.0:8090/web`。启用 GitHub 认证时传入环境变量，例如：

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

---

## 其他

- 建议以 **root** 运行（多用户模式下需切换进程 UID/GID）。
- 命令行参数：`-bind`、`-enable-ssl`、`-fork`、`-debug`、`-single` 等，可通过 `./wsterm -h` 查看。

# ws-shell

[![Deploy with Vercel](https://vercel.com/button)](https://vercel.com/new/clone?repository-url=https://github.com/v1xingyue/ws-shell)

English | [中文](readme.zh-CN.md)

WebSocket-based web terminal with optional GitHub OAuth authentication.

## Features

- Run a real interactive shell in the browser through WebSocket + PTY.
- Open the terminal at `/web`; the root path redirects there.
- Send keyboard input, receive terminal output, and resize the PTY from the browser.
- Choose the shell command with `-fork`, for example `/bin/bash`, `/bin/zsh`, or `/bin/sh`.
- Run in single-user mode by default, or switch users with `?user=username` when multi-user mode is enabled.
- Protect access with optional GitHub OAuth.
- Restrict login to specific GitHub numeric user IDs with `ALLOWED_USER_IDS`.
- Load configuration from environment variables or a root `.env` file.
- Serve with built-in HTTPS locally, or disable app TLS behind a platform HTTPS proxy.
- Build and run as a small Docker image.

## What the Vercel Dockerfile Deploy Gives You

The deploy button points Vercel at this repository. `Dockerfile.vercel` builds the Vite web UI, builds the Go server, and starts:

```bash
/app/bin/wsterm -bind :80 -fork /bin/sh
```

That gives you:

- A browser terminal at `https://YOUR_DEPLOYMENT/web`.
- WebSocket terminal sessions backed by `/bin/sh` inside the deployed Alpine container.
- HTTPS at the public URL, with app-level SSL disabled because the platform terminates TLS.
- Optional GitHub OAuth protection through Vercel environment variables:
  - `GITHUB_CLIENT_ID`
  - `GITHUB_CLIENT_SECRET`
  - `ALLOWED_USER_IDS`
  - `OAUTH_REDIRECT_URL`
- A quick disposable shell for inspecting the deployed container environment, testing network access, checking environment variables, and running bundled tools.

Limits to expect:

- The shell runs inside the deployment container, not on your local machine.
- Files and processes are not durable across redeploys, restarts, or container replacement.
- You only get what exists in the image; add packages in `Dockerfile.vercel` if you need more tools.
- Do not expose it publicly without OAuth and an allowlist.

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
| `OAUTH_LOCAL_DEV` | Set to `true` to start OAuth through the public callback and bounce the one-time code back to local dev. |
| `OAUTH_LOCAL_REDIRECT_URL` | Public OAuth callback used in local dev. Default: `https://ws-shell.vercel.app/auth/github/callback?local=true`. |
| `OAUTH_LOCAL_CALLBACK_URL` | Local callback target for the public bounce. Default: `http://localhost/auth/github/callback`. |
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
- For local debugging with the fixed production OAuth callback, set `OAUTH_LOCAL_DEV=true`; the public callback redirects the one-time GitHub code back to `OAUTH_LOCAL_CALLBACK_URL`.

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

Use the published image:

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

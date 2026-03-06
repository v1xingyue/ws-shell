# ws-shell

A shell running based on websocket with optional GitHub OAuth authentication.

## Features

- WebSocket-based terminal emulator
- Optional GitHub OAuth authentication (only enabled when credentials are provided)
- User access control via allowed user IDs (when authentication is enabled)
- SSL support

## Build and compile

```bash
cd web && pnpm install && pnpm run build
go build
```

## Run

You can copy the binary to any server and run it.

```bash
./wsterm
```

### Without Authentication (Default)

By default, the application runs without authentication. Simply run:

```bash
./wsterm
```

### With GitHub Authentication (Optional)

To enable GitHub OAuth authentication, set the following environment variables:

```bash
export GITHUB_CLIENT_ID=your_github_client_id
export GITHUB_CLIENT_SECRET=your_github_client_secret
export ALLOWED_USER_IDS=12345678,87654321  # Optional: restrict access to specific users

./wsterm
```

See [AUTH_SETUP.md](AUTH_SETUP.md) for detailed configuration instructions.

## Tips

this must running with root user.

## Run with docker

```bash
docker run -d --name ghcr.io/v1xingyue/ws-shell:main -p 8090:8080 
```

After running, you can access the service through the address: [https://0.0.0.0:8090](https://0.0.0.0:8090)


## Run with red-pill-shell

```bash
docker run -d --name ghcr.io/v1xingyue/ws-shell:main -e red-pill-token=true -p 8090:8080 
```

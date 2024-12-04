# ws-shell

A shell running based on websocket.

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

## Tips

this must running with root user.

## Run with docker

```bash
docker run -d --name wsterm -p 8090:8080 ws-shell
```

After running, you can access the service through the address: [https://0.0.0.0:8090](https://0.0.0.0:8090)

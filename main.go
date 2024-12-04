package main

import (
	"encoding/json"
	"flag"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var (
	enableSSL = os.Getenv("ENABLE_SSL") == "true"
	upgrader  = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

func getDefaultIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

// WSMessage 定义WebSocket消息结构
type WSMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
		ForceQuote:    false,
	})
	logrus.SetLevel(logrus.InfoLevel)
}

func wsHandler(c *gin.Context) {
	// 首先创建 WebSocket 连接
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logrus.Errorf("WebSocket upgrade failed: %v", err)
		return
	}
	defer ws.Close()

	// 定义一个辅助函数来发送错误消息
	sendMsg := func(errMsg string) {
		ws.WriteMessage(websocket.TextMessage, []byte(errMsg+"\n"))
	}

	username := ""
	if singleUserMode {
		username = defaultUser
	} else {
		username = c.DefaultQuery("user", defaultUser)
	}

	logrus.Infof("run as user: %s", username)

	targetUser, err := user.Lookup(username)
	if err != nil {
		errMsg := "Failed to lookup user: " + err.Error()
		logrus.Error(errMsg)
		sendMsg(errMsg)
		sendMsg("You can visit ?user=username to change user")
		return
	}

	// 转换 uid 和 gid 为整数
	uid, err := strconv.ParseUint(targetUser.Uid, 10, 32)
	if err != nil {
		errMsg := "Failed to parse UID: " + err.Error()
		logrus.Error(errMsg)
		sendMsg(errMsg)
		return
	}
	gid, err := strconv.ParseUint(targetUser.Gid, 10, 32)
	if err != nil {
		errMsg := "Failed to parse GID: " + err.Error()
		logrus.Error(errMsg)
		sendMsg(errMsg)
		sendMsg("You can visit ?user=username to change user")
		return
	}

	cmd := exec.Command(forkCmd, "-l")
	cmd.Env = append(os.Environ(),
		"USER="+targetUser.Username,
		"HOME="+targetUser.HomeDir,
		"PATH="+os.Getenv("PATH"),
	)
	cmd.Env = append(cmd.Env, autoEnv...)

	if !singleUserMode {
		// 设置进程的 uid 和 gid
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: uint32(uid),
				Gid: uint32(gid),
			},
		}
	}

	// 写入临时的 .bashrc 文件
	tmpBashRc := os.TempDir() + "/.bashrc_" + c.Request.RemoteAddr
	if err := os.WriteFile(tmpBashRc, []byte(bashRcContent), 0644); err != nil {
		logrus.Errorf("Failed to create temporary .bashrc: %v", err)
		return
	}
	defer os.Remove(tmpBashRc)

	// 设置 BASH_ENV 来加载我们的配置
	cmd.Env = append(cmd.Env, "BASH_ENV="+tmpBashRc)

	// 启动PTY
	f, err := pty.Start(cmd)
	if err != nil {
		errMsg := "Failed to start PTY: " + err.Error()
		logrus.Error(errMsg)
		sendMsg(errMsg)
		return
	}
	defer f.Close()

	// 从PTY读取并发送到WebSocket
	go func() {
		buffer := make([]byte, 1024)
		for {
			n, err := f.Read(buffer)
			if err != nil {
				if err != io.EOF {
					errMsg := "Failed to read from PTY: " + err.Error()
					logrus.Error(errMsg)
					sendMsg(errMsg)
				}
				return
			}
			if err := ws.WriteMessage(websocket.TextMessage, buffer[:n]); err != nil {
				errMsg := "Failed to send to WebSocket: " + err.Error()
				logrus.Error(errMsg)
				sendMsg(errMsg)
				return
			}
		}
	}()

	// 从WebSocket读取并处理消息
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			logrus.Errorf("Failed to read WebSocket message: %v", err)
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			logrus.Errorf("Failed to parse message: %v", err)
			continue
		}

		switch msg.Type {
		case "input":
			// 写入用户输入到PTY
			if _, err := f.Write([]byte(msg.Data)); err != nil {
				logrus.Errorf("Failed to write to PTY: %v", err)
			}
		case "resize":
			// 调整终端大小
			if err := pty.Setsize(f, &pty.Winsize{
				Rows: msg.Rows,
				Cols: msg.Cols,
			}); err != nil {
				logrus.Errorf("Failed to resize terminal: %v", err)
			}
		}
	}

	logrus.Info("Closing connection, terminating process")
	cmd.Process.Kill()

}

var version = "unknown"

func main() {
	flag.StringVar(&bindAddress, "bind", bindAddress, "bind address")
	flag.BoolVar(&debugMode, "debug", debugMode, "run in debug mode")
	flag.BoolVar(&singleUserMode, "single", singleUserMode, "run as single user mode")
	flag.BoolVar(&enableSSL, "enable-ssl", enableSSL, "enable ssl")
	flag.StringVar(&version, "version", version, "show version")
	flag.StringVar(&forkCmd, "fork", forkCmd, "fork command")
	flag.Parse()

	if enableSSL {
		logrus.Info("Running with SSL")
		generateCert()
	}

	if singleUserMode {
		logrus.Info("Single user mode")
	} else {
		logrus.Info("Multi user mode")
	}

	logrus.Info("version: ", version)

	if debugMode {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()

	// WebSocket 路由
	r.GET("/ws", wsHandler)

	Setup(r)

	// 重定向根路径到 /web
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/web")
	})

	logrus.Info("Server started on port ", bindAddress)

	defaultHost := getDefaultIP()
	var visitUrl string
	if enableSSL {
		visitUrl = "https://" + defaultHost + bindAddress + "/web"
	} else {
		visitUrl = "http://" + defaultHost + bindAddress + "/web"
	}
	logrus.Infof("You may visit %s to use the terminal", visitUrl)

	var err error

	if enableSSL {
		err = r.RunTLS(bindAddress, "cert.pem", "key.pem")
	} else {
		err = r.Run(bindAddress)
	}

	if err != nil {
		logrus.Fatalf("Failed to start server: %v", err)
	}
}

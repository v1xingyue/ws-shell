import { useEffect, useRef } from "react";
import { Terminal as XTerm } from "xterm";
import { FitAddon } from "xterm-addon-fit";
import { WebglAddon } from "xterm-addon-webgl";
import "xterm/css/xterm.css";

// 定义消息接口
interface TerminalMessage {
  type: "command" | "resize";
  command?: string;
  cols?: number;
  rows?: number;
}

const Terminal = () => {
  const terminalRef = useRef<HTMLDivElement>(null);
  const terminalInstance = useRef<XTerm | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const params = new URLSearchParams(window.location.search);
  const username = params.get("user") || "staff";

  useEffect(() => {
    // Initialize terminal
    const term = new XTerm({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      theme: {
        background: "#1e1e1e",
        foreground: "#f0f0f0",
        cursor: "#f0f0f0",
        // Dracula theme colors
        black: "#21222C",
        red: "#FF5555",
        green: "#50FA7B",
        yellow: "#F1FA8C",
        blue: "#BD93F9",
        magenta: "#FF79C6",
        cyan: "#8BE9FD",
        white: "#F8F8F2",
        brightBlack: "#6272A4",
        brightRed: "#FF6E6E",
        brightGreen: "#69FF94",
        brightYellow: "#FFFFA5",
        brightBlue: "#D6ACFF",
        brightMagenta: "#FF92DF",
        brightCyan: "#A4FFFF",
        brightWhite: "#FFFFFF",
      },
      convertEol: true,
      scrollback: 1000,
      allowTransparency: true,
    });

    terminalInstance.current = term;

    // Add WebGL addon
    const webglAddon = new WebglAddon();
    term.loadAddon(webglAddon);

    // Add FitAddon
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);

    // Open terminal in container
    if (terminalRef.current) {
      term.open(terminalRef.current);
      fitAddon.fit();
    }

    // Connect WebSocket
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(
      `${protocol}//${window.location.host}/ws?user=${username}`
    );
    wsRef.current = ws;

    // 添加 postMessage 监听器
    const handleMessage = (event: MessageEvent) => {
      console.log("Terminal received message:", event.data); // 调试日志

      // 验证消息来源
      if (!event.origin.includes(window.location.host)) {
        console.log("Origin check failed:", event.origin); // 调试日志
        return;
      }

      try {
        const message: TerminalMessage =
          typeof event.data === "string" ? JSON.parse(event.data) : event.data;

        console.log("Processed message:", message); // 调试日志

        if (message.type === "command" && message.command) {
          console.log("Sending command to WebSocket:", message.command); // 调试日志

          if (wsRef.current?.readyState === WebSocket.OPEN) {
            wsRef.current.send(
              JSON.stringify({
                type: "input",
                data: message.command,
              })
            );
            console.log("Command sent to WebSocket"); // 调试日志
          } else {
            console.log("WebSocket not ready:", wsRef.current?.readyState); // 调试日志
          }
        }

        if (message.type === "resize" && message.cols && message.rows) {
          // 处理调整大小的请求
          if (terminalInstance.current) {
            terminalInstance.current.resize(message.cols, message.rows);
          }
        }
      } catch (error) {
        console.error("Failed to parse postMessage:", error);
      }
    };

    window.addEventListener("message", handleMessage);

    // Handle WebSocket events
    ws.onmessage = (event) => {
      // 写入到终端
      if (terminalInstance.current) {
        terminalInstance.current.write(event.data);
      }

      // 将结果发送回父窗口
      window.parent.postMessage(
        {
          type: "terminal-output",
          data: event.data,
        },
        "*"
      );
    };

    ws.onclose = () => {
      term.write("\r\n\x1b[31mConnection closed\x1b[0m\r\n");
    };

    // Handle terminal input
    term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(
          JSON.stringify({
            type: "input",
            data: data,
          })
        );
      }
    });

    // Send terminal dimensions on connection
    ws.onopen = () => {
      ws.send(
        JSON.stringify({
          type: "resize",
          cols: term.cols,
          rows: term.rows,
        })
      );
    };

    // Handle window resize
    const handleResize = () => {
      fitAddon.fit();
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(
          JSON.stringify({
            type: "resize",
            cols: term.cols,
            rows: term.rows,
          })
        );
      }
    };

    window.addEventListener("resize", handleResize);

    // Error handling for WebGL
    webglAddon.onContextLoss(() => {
      webglAddon.dispose();
    });

    // Cleanup
    return () => {
      window.removeEventListener("resize", handleResize);
      window.removeEventListener("message", handleMessage);
      webglAddon.dispose();
      term.dispose();
      ws.close();
    };
  }, []);

  return <div ref={terminalRef} style={{ width: "100%", height: "100vh" }} />;
};

export default Terminal;

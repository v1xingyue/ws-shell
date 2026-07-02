import { type CSSProperties, useCallback, useEffect, useRef } from "react";
import { Terminal as WTerm, type TerminalHandle } from "@wterm/react";
import "@wterm/react/css";

// 定义消息接口
interface TerminalMessage {
  type: "command" | "resize";
  command?: string;
  cols?: number;
  rows?: number;
}

const Terminal = () => {
  const terminalRef = useRef<TerminalHandle>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const params = new URLSearchParams(window.location.search);
  const username = params.get("user") || "staff";

  const sendResize = useCallback((cols: number, rows: number) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(
        JSON.stringify({
          type: "resize",
          cols,
          rows,
        })
      );
    }
  }, []);

  const handleData = useCallback((data: string) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(
        JSON.stringify({
          type: "input",
          data,
        })
      );
    }
  }, []);

  useEffect(() => {
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
          terminalRef.current?.resize(message.cols, message.rows);
          sendResize(message.cols, message.rows);
        }
      } catch (error) {
        console.error("Failed to parse postMessage:", error);
      }
    };

    window.addEventListener("message", handleMessage);

    // Handle WebSocket events
    ws.onmessage = (event) => {
      // 写入到终端
      terminalRef.current?.write(event.data);

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
      terminalRef.current?.write("\r\n\x1b[31mConnection closed\x1b[0m\r\n");
    };

    // Cleanup
    return () => {
      window.removeEventListener("message", handleMessage);
      ws.close();
    };
  }, [sendResize, username]);

  const terminalStyle = {
    width: "100%",
    height: "100vh",
    boxSizing: "border-box",
    borderRadius: 0,
    boxShadow: "none",
    "--term-bg": "#1e1e1e",
    "--term-fg": "#f0f0f0",
    "--term-cursor": "#f0f0f0",
    "--term-font-family": 'Menlo, Monaco, "Courier New", monospace',
    "--term-font-size": "14px",
  } as CSSProperties;

  return (
    <WTerm
      ref={terminalRef}
      autoResize
      cursorBlink
      onData={handleData}
      onReady={(term) => sendResize(term.cols, term.rows)}
      onResize={sendResize}
      style={terminalStyle}
    />
  );
};

export default Terminal;

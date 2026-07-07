package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpInitializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
}

type mcpSSESession struct {
	ch chan any
}

var mcpSSESessions sync.Map

func setupMCPRoutes(r *gin.Engine) {
	group := r.Group("/console/:name/mcp", MCPAuthMiddleware())
	group.GET("", func(c *gin.Context) {
		if strings.Contains(c.GetHeader("Accept"), "text/event-stream") {
			c.Status(http.StatusMethodNotAllowed)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"name":      c.Param("name"),
			"protocol":  "mcp",
			"transport": "streamable-http",
		})
	})
	group.POST("", handleMCP)
	group.GET("/sse", handleMCPSSE)
	group.POST("/message", handleMCPSSEMessage)
}

func MCPAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(os.Getenv("MCP_TOKEN"))
		if token == "" {
			c.JSON(http.StatusNotFound, gin.H{
				"error":  "MCP disabled",
				"reason": "MCP_TOKEN is empty",
			})
			c.Abort()
			return
		}

		got := ""
		if parts := strings.Fields(c.GetHeader("Authorization")); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			got = parts[1]
		}
		if got == "" {
			got = c.Query("token")
		}
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid MCP token"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func handleMCP(c *gin.Context) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		mcpRespond(c, nil, nil, &mcpError{Code: -32700, Message: "Parse error"})
		return
	}
	resp, ok, err := mcpHTTPResponse(raw)
	if err != nil {
		mcpRespond(c, nil, nil, &mcpError{Code: -32700, Message: "Parse error"})
		return
	}
	if !ok {
		c.Status(http.StatusAccepted)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func handleMCPSSE(c *gin.Context) {
	sessionID, err := newMCPSessionID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create MCP session"})
		return
	}
	session := &mcpSSESession{ch: make(chan any, 8)}
	mcpSSESessions.Store(sessionID, session)
	defer mcpSSESessions.Delete(sessionID)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)

	query := c.Request.URL.RawQuery
	if query != "" {
		query += "&"
	}
	query += "session=" + sessionID
	endpoint := fmt.Sprintf("%s/console/%s/mcp/message?%s", requestBaseURL(c), c.Param("name"), query)
	writeSSE(c.Writer, "endpoint", endpoint)
	c.Writer.Flush()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case resp := <-session.ch:
			data, _ := json.Marshal(resp)
			writeSSE(c.Writer, "message", string(data))
			c.Writer.Flush()
		}
	}
}

func handleMCPSSEMessage(c *gin.Context) {
	rawSession, ok := mcpSSESessions.Load(c.Query("session"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP SSE session not found"})
		return
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		rawSession.(*mcpSSESession).ch <- mcpErrorResponse(nil, &mcpError{Code: -32700, Message: "Parse error"})
		c.Status(http.StatusAccepted)
		return
	}
	resp, ok, err := mcpHTTPResponse(raw)
	if err != nil {
		rawSession.(*mcpSSESession).ch <- mcpErrorResponse(nil, &mcpError{Code: -32700, Message: "Parse error"})
		c.Status(http.StatusAccepted)
		return
	}
	if ok {
		rawSession.(*mcpSSESession).ch <- resp
	}
	c.Status(http.StatusAccepted)
}

func mcpHTTPResponse(raw []byte) (any, bool, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil, false, errors.New("empty request")
	}
	if raw[0] == '[' {
		var reqs []mcpRequest
		if err := json.Unmarshal(raw, &reqs); err != nil {
			return nil, false, err
		}
		responses := []gin.H{}
		for _, req := range reqs {
			if resp, ok := mcpResponse(req); ok {
				responses = append(responses, resp)
			}
		}
		if len(responses) == 0 {
			return nil, false, nil
		}
		return responses, true, nil
	}

	var req mcpRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, false, err
	}
	resp, ok := mcpResponse(req)
	return resp, ok, nil
}

func mcpResponse(req mcpRequest) (gin.H, bool) {
	if req.ID == nil && strings.HasPrefix(req.Method, "notifications/") {
		return nil, false
	}

	switch req.Method {
	case "initialize":
		var params mcpInitializeParams
		_ = json.Unmarshal(req.Params, &params)
		return mcpSuccessResponse(req.ID, gin.H{
			"protocolVersion": mcpProtocolVersion(params.ProtocolVersion),
			"serverInfo": gin.H{
				"name":    "ws-shell",
				"version": version,
			},
			"capabilities": gin.H{
				"tools": gin.H{},
			},
		}), true
	case "ping":
		return mcpSuccessResponse(req.ID, gin.H{}), true
	case "tools/list":
		return mcpSuccessResponse(req.ID, gin.H{
			"tools": []gin.H{{
				"name":        "shell",
				"description": "Run a shell command on this VM.",
				"inputSchema": gin.H{
					"type": "object",
					"properties": gin.H{
						"command": gin.H{"type": "string"},
						"cwd": gin.H{
							"type":        "string",
							"description": "Optional working directory.",
						},
						"timeout_ms": gin.H{
							"type":        "number",
							"description": "Optional timeout, capped at 30000.",
						},
					},
					"required": []string{"command"},
				},
			}},
		}), true
	case "resources/list":
		return mcpSuccessResponse(req.ID, gin.H{"resources": []gin.H{}}), true
	case "prompts/list":
		return mcpSuccessResponse(req.ID, gin.H{"prompts": []gin.H{}}), true
	case "tools/call":
		result, err := callMCPTool(req.Params)
		if err != nil {
			return mcpErrorResponse(req.ID, &mcpError{Code: -32602, Message: err.Error()}), true
		}
		return mcpSuccessResponse(req.ID, result), true
	default:
		return mcpErrorResponse(req.ID, &mcpError{Code: -32601, Message: "Method not found"}), true
	}
}

func callMCPTool(raw json.RawMessage) (gin.H, error) {
	var params struct {
		Name      string `json:"name"`
		Arguments struct {
			Command   string `json:"command"`
			Cwd       string `json:"cwd"`
			TimeoutMS int    `json:"timeout_ms"`
		} `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if params.Name != "shell" {
		return nil, errors.New("unknown tool")
	}
	if params.Arguments.Command == "" {
		return nil, errors.New("command is required")
	}
	if params.Arguments.TimeoutMS <= 0 || params.Arguments.TimeoutMS > 30000 {
		params.Arguments.TimeoutMS = 30000
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(params.Arguments.TimeoutMS)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, forkCmd, "-lc", params.Arguments.Command)
	if params.Arguments.Cwd != "" {
		cmd.Dir = params.Arguments.Cwd
	}
	cmd.Env = append(os.Environ(), autoEnv...)
	out, err := cmd.CombinedOutput()
	isError := err != nil
	if ctx.Err() == context.DeadlineExceeded {
		isError = true
		out = append(out, []byte("\ncommand timed out")...)
	}

	return gin.H{
		"content": []gin.H{{
			"type": "text",
			"text": string(out),
		}},
		"isError": isError,
	}, nil
}

func mcpRespond(c *gin.Context, id json.RawMessage, result any, rpcErr *mcpError) {
	c.JSON(http.StatusOK, mcpJSONResponse(id, result, rpcErr))
}

func mcpSuccessResponse(id json.RawMessage, result any) gin.H {
	return mcpJSONResponse(id, result, nil)
}

func mcpErrorResponse(id json.RawMessage, rpcErr *mcpError) gin.H {
	return mcpJSONResponse(id, nil, rpcErr)
}

func mcpJSONResponse(id json.RawMessage, result any, rpcErr *mcpError) gin.H {
	resp := gin.H{"jsonrpc": "2.0"}
	if id != nil {
		resp["id"] = json.RawMessage(id)
	}
	if rpcErr != nil {
		resp["error"] = rpcErr
	} else {
		resp["result"] = result
	}
	return resp
}

func mcpProtocolVersion(requested string) string {
	switch requested {
	case "2025-06-18", "2025-03-26", "2024-11-05":
		return requested
	default:
		return "2025-06-18"
	}
}

func newMCPSessionID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func writeSSE(w http.ResponseWriter, event, data string) {
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	for line := range strings.SplitSeq(data, "\n") {
		_, _ = fmt.Fprintf(w, "data: %s\n", line)
	}
	_, _ = fmt.Fprint(w, "\n")
}

func requestBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.GetHeader("X-Forwarded-Proto") == "https" || c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

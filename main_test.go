package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestMCPRoutes(t *testing.T) *gin.Engine {
	t.Helper()
	t.Setenv("MCP_TOKEN", "secret")
	r := gin.New()
	setupMCPRoutes(r)
	return r
}

func TestMCPToolRunsShellCommand(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldAuth := authEnabled
	authEnabled = false
	oldFork := forkCmd
	forkCmd = "/bin/sh"
	defer func() { authEnabled = oldAuth }()
	defer func() { forkCmd = oldFork }()

	r := setupTestMCPRoutes(t)

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"shell","arguments":{"command":"printf ok","timeout_ms":1000}}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/console/vm/mcp?token=secret", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ok") {
		t.Fatalf("body = %s, want shell output", w.Body.String())
	}
}

func TestMCPInitializeEchoesSupportedProtocolVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := setupTestMCPRoutes(t)

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/console/vm/mcp?token=secret", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"protocolVersion":"2025-06-18"`) {
		t.Fatalf("body = %s, want requested protocol version", w.Body.String())
	}
}

func TestMCPInitializeSupportsLegacyProtocolVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := setupTestMCPRoutes(t)

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/console/vm/mcp?token=secret", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"protocolVersion":"2024-11-05"`) {
		t.Fatalf("body = %s, want requested protocol version", w.Body.String())
	}
}

func TestMCPBatchRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := setupTestMCPRoutes(t)

	body := []byte(`[{"jsonrpc":"2.0","id":1,"method":"ping","params":{}},{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}]`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/console/vm/mcp?token=secret", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.HasPrefix(w.Body.String(), "[") || !strings.Contains(w.Body.String(), `"tools"`) {
		t.Fatalf("body = %s, want batch response", w.Body.String())
	}
}

func TestMCPCommonProbeMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		method string
		want   string
	}{
		{"ping", `"result":{}`},
		{"resources/list", `"resources":[]`},
		{"prompts/list", `"prompts":[]`},
	} {
		r := setupTestMCPRoutes(t)

		body := []byte(`{"jsonrpc":"2.0","id":1,"method":"` + tc.method + `","params":{}}`)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/console/vm/mcp?token=secret", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", tc.method, w.Code)
		}
		if !strings.Contains(w.Body.String(), tc.want) {
			t.Fatalf("%s body = %s, want %s", tc.method, w.Body.String(), tc.want)
		}
	}
}

func TestNoRouteProxiesToLocalWebService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"path": r.URL.Path})
	}))
	defer upstream.Close()

	oldTarget := webProxyTarget
	webProxyTarget = upstream.URL
	defer func() { webProxyTarget = oldTarget }()

	r := gin.New()
	setupWebProxy(r)
	server := httptest.NewServer(r)
	defer server.Close()

	resp, err := http.Get(server.URL + "/app/page")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body := new(bytes.Buffer)
	_, _ = body.ReadFrom(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body.String(), `"/app/page"`) {
		t.Fatalf("body = %s, want proxied path", body.String())
	}
}

func TestBackgroundServerURLUsesEnv(t *testing.T) {
	t.Setenv("BACKGROUND_SERVER_URL", "http://127.0.0.1:3999")

	if got := backgroundServerURL(); got != "http://127.0.0.1:3999" {
		t.Fatalf("target = %q, want env value", got)
	}
}

func TestProxyShowsBackgroundServerMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	target := upstream.URL
	upstream.Close()

	oldTarget := webProxyTarget
	webProxyTarget = target
	defer func() { webProxyTarget = oldTarget }()

	r := gin.New()
	setupWebProxy(r)
	server := httptest.NewServer(r)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body := new(bytes.Buffer)
	_, _ = body.ReadFrom(resp.Body)

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
	if !strings.Contains(body.String(), "Background server not running") {
		t.Fatalf("body = %s, want background server message", body.String())
	}
}

func TestConsoleAndMCPRoutesCanRegisterTogether(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	Setup(r)
	t.Setenv("MCP_TOKEN", "secret")
	setupMCPRoutes(r)
	setupWebProxy(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/console/vm/mcp?token=secret", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"protocol":"mcp"`) {
		t.Fatalf("body = %s, want mcp response", w.Body.String())
	}
}

func TestBareMCPRouteExplainsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := setupTestMCPRoutes(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/console/vm/mcp?token=secret", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"transport":"streamable-http"`) {
		t.Fatalf("body = %s, want streamable-http hint", w.Body.String())
	}
}

func TestMCPDisabledWithoutToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("MCP_TOKEN", "")

	r := gin.New()
	setupMCPRoutes(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/console/vm/mcp", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestMCPStreamableGetOpensSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := setupTestMCPRoutes(t)
	server := httptest.NewServer(r)
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/console/vm/mcp?token=secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
}

func TestLegacyMCPSSEEndpointAdvertisesMessageURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := setupTestMCPRoutes(t)
	server := httptest.NewServer(r)
	defer server.Close()

	resp, err := http.Get(server.URL + "/console/vm/mcp/sse?token=secret")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	line1, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	line2, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(line1, "event: endpoint") || !strings.Contains(line2, server.URL+"/console/vm/mcp/message?token=secret&session=") {
		t.Fatalf("sse = %q %q, want endpoint event", line1, line2)
	}
}

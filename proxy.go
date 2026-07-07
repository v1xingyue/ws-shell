package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var webProxyTarget = backgroundServerURL()

func backgroundServerURL() string {
	if target := strings.TrimSpace(os.Getenv("BACKGROUND_SERVER_URL")); target != "" {
		return target
	}
	return "http://localhost:3000"
}

func setupWebProxy(r *gin.Engine) {
	target, err := url.Parse(webProxyTarget)
	if err != nil {
		logrus.Fatalf("Invalid web proxy target: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		logrus.WithError(err).WithField("target", webProxyTarget).Warn("background server unavailable")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = fmt.Fprintf(w, "<h1>Background server not running</h1><p>Target: %s</p><p>Start it, or set BACKGROUND_SERVER_URL.</p>", webProxyTarget)
	}

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/console/") {
			serveConsolePath(c, strings.TrimPrefix(c.Request.URL.Path, "/console/"))
			return
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	})
}

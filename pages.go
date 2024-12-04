package main

import (
	"embed"
	"mime"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

//go:embed web/dist
var webContent embed.FS

func Setup(r *gin.Engine) {

	r.GET("/web/*path", func(c *gin.Context) {
		path := c.Param("path")
		if path == "/" {
			path = "index.html"
		}

		lookUpPath := filepath.Join(webDistPrefix, path)
		logrus.WithField("prefix", webDistPrefix).WithField("path", lookUpPath).Debug("lookup file")
		content, err := webContent.ReadFile(lookUpPath)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}

		// auto set content type
		suffix := filepath.Ext(lookUpPath)
		contentType := mime.TypeByExtension(suffix)
		logrus.WithField("suffix", suffix).WithField("contentType", contentType).Debug("set content type")
		c.Data(http.StatusOK, contentType, content)
	})

}

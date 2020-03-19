package httplive

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware ...
// nolint lll
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		s := c.Writer.Header().Set
		s("Access-Control-Allow-Origin", "*")
		s("Access-Control-Max-Age", "86400")
		s("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		s("Access-Control-Allow-Headers", "X-Requested-With, Content-Type, Origin, Authorization, Accept, Client-Security-Token, Accept-Encoding, x-access-token")
		s("Access-Control-Expose-Headers", "Content-Length")
		s("Access-Control-Allow-Credentials", "true")
		s("Cache-Control", "no-cache, no-store, must-revalidate")
		s("Pragma", "no-cache")
		s("Expires", "0")

		if c.Request.Method == http.MethodOptions {
			logrus.Infof(http.MethodOptions)
			c.AbortWithStatus(http.StatusOK)
		} else {
			c.Next()
		}
	}
}

// StaticFileMiddleware ...
func StaticFileMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		uriPath := c.Request.URL.Path
		assetPath := "public" + uriPath

		if c.Request.Method == http.MethodGet && uriPath == "/" {
			assetPath = "public/index.html"
		}

		if path.Ext(assetPath) == ".map" {
			c.Status(http.StatusNotFound)
			c.Abort()

			return
		}

		if os.Getenv("debug") != "" {
			TryGetLocalFile(c, assetPath)
		} else {
			TryGetAssetFile(c, assetPath)
		}

		if c.IsAborted() {
			return
		}

		c.Next()
	}
}

// APIMiddleware ...
func APIMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if p == "/" || p == "/config.js" ||
			strings.HasPrefix(p, "/ws") ||
			strings.HasPrefix(p, "/webcli/") ||
			strings.HasPrefix(p, "/fonts/") ||
			strings.HasPrefix(p, "/app/") ||
			strings.HasPrefix(p, "/css/") ||
			strings.HasPrefix(p, "/img/") ||
			strings.HasPrefix(p, "/components/") ||
			strings.HasPrefix(p, "/vendor/") {
			c.Next()

			return
		}

		if EndpointServeHTTP(c.Writer, c.Request) {
			if boradcastThrottler.Allow() {
				Broadcast(c)
			}

			c.Abort()
		} else {
			c.Next()
		}
	}
}

// ConfigJsMiddleware ...
func ConfigJsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path != "/config.js" {
			c.Next()

			return
		}

		fileContent := fmt.Sprintf(`
define('config', { 
	defaultPort:'%s', 
	savePath: '/webcli/api/save', 
	fetchPath: '/webcli/api/endpoint', 
	deletePath: '/webcli/api/deleteendpoint', 
	treePath: '/webcli/api/tree', 
	componentId: ''
});`, Environments.DefaultPort)
		c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileContent)))
		c.Writer.Header().Set("Content-Type", "application/javascript")
		c.String(http.StatusOK, fileContent)
		c.Abort()
	}
}

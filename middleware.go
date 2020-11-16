package httplive

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware ...
// nolint lll
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Max-Age", "86400")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		c.Header("Access-Control-Allow-Headers", "X-Requested-With, Content-Type, Origin, Authorization, Accept, Client-Security-Token, Accept-Encoding, x-access-token")
		c.Header("Access-Control-Expose-Headers", "Content-Length")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")

		if c.Request.Method == http.MethodOptions {
			logrus.Infof(http.MethodOptions)
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// StaticFileMiddleware ...
func StaticFileMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := TrimContextPath(c)
		if HasAnyPrefix(p, "/httplive/webcli", "/httplive/ws") {
			c.Next()
			return
		}

		uriPath := strings.TrimPrefix(p, "/httplive")
		assetPath := "public" + uriPath
		if c.Request.Method == http.MethodGet && uriPath == "/" {
			assetPath = "public/index.html"
		}

		if TryGetFile(c, assetPath) {
			c.Abort()
			return
		}

		c.Next()
	}
}

// HasAnyPrefix tells that s has prefix of any prefixes.
func HasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}

	return false
}

// APIMiddleware ...
func APIMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := TrimContextPath(c)
		if p == "/" || p == "/favicon.ico" || strings.HasPrefix(p, "/httplive/") {
			c.Next()
			return
		}

		if result := EndpointServeHTTP(c.Writer, c.Request); result.RouterServed {
			if boradcastThrottler.Allow() {
				Broadcast(c, result.RouterBody)
			}

			c.Abort()
			return
		}

		c.Next()
	}
}

func TrimContextPath(c *gin.Context) string {
	p := c.Request.URL.Path
	if Environments.ContextPath != "/" {
		p = strings.TrimPrefix(p, Environments.ContextPath)
	}

	if p == "" {
		return "/"
	}

	return p
}

// ConfigJsMiddleware ...
func ConfigJsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := TrimContextPath(c)
		if p != "/httplive/config.js" {
			c.Next()
			return
		}

		fileContent := ReplaceContextPathString(fmt.Sprintf(`
define('httplive/config', {
	defaultPort:'%s',
	savePath: '${ContextPath}/httplive/webcli/api/save',
	fetchPath: '${ContextPath}/httplive/webcli/api/endpoint',
	deletePath: '${ContextPath}/httplive/webcli/api/deleteendpoint',
	treePath: '${ContextPath}/httplive/webcli/api/tree',
	componentId: ''
});`, Environments.Ports))
		c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileContent)))
		c.Writer.Header().Set("Content-Type", "application/javascript")
		c.String(http.StatusOK, fileContent)
		c.Abort()
	}
}

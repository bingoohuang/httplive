package httplive

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/bingoohuang/httplive/internal/process"

	"github.com/bingoohuang/httplive/internal/res"
	"github.com/bingoohuang/httplive/pkg/util"

	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

// StaticFileMiddleware ...
func StaticFileMiddleware(c *gin.Context) {
	p := trimContextPath(c)
	if util.HasPrefix(p, "/httplive/webcli", "/httplive/ws") {
		c.Next()
		return
	}

	uriPath := strings.TrimPrefix(p, "/httplive")
	assetPath := filepath.Join("/public", uriPath)
	if c.Request.Method == http.MethodGet && uriPath == "/" {
		assetPath = "/public/index.html"
	}

	if res.TryGetFile(c, assetPath, Environments.ContextPath) {
		c.Abort()
		return
	}

	c.Next()
}

// APIMiddleware ...
func APIMiddleware(c *gin.Context) {
	p := trimContextPath(c)
	if util.AnyOf(p, "/", "/favicon.ico") || util.HasPrefix(p, "/httplive/") {
		c.Next()
		return
	}

	if result := serveAPI(c.Writer, c.Request); result.RouterServed {
		if broadcastThrottler.Allow() {
			broadcast(c, result)
		}

		c.Abort()
		return
	}

	c.Next()
}

func broadcast(c *gin.Context, rr process.RouterResult) {
	msg := process.WsMessage{
		Time:   util.TimeFmt(time.Now()),
		Host:   c.Request.Host,
		Body:   util.GetRequestBody(c),
		Method: c.Request.Method,
		Path:   c.Request.URL.Path,
		Query:  util.ConvertHeader(c.Request.URL.Query()),
		Header: util.GetHeaders(c),

		Response:       util.CompactJSON(rr.RouterBody),
		ResponseSize:   rr.ResponseSize,
		ResponseStatus: rr.ResponseStatus,
		ResponseHeader: rr.ResponseHeader,
		RemoteAddr:     rr.RemoteAddr,
	}

	for id, conn := range Clients {
		if err := conn.WriteJSON(msg); err != nil {
			logrus.Warnf("conn WriteJSON error: %v", err)

			conn.Close()

			delete(Clients, id)
		}
	}
}

func trimContextPath(c *gin.Context) string {
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
func ConfigJsMiddleware(c *gin.Context) {
	p := trimContextPath(c)
	if p != "/httplive/config.js" {
		c.Next()
		return
	}

	fileContent := res.ReplaceContextPath([]byte(fmt.Sprintf(`
define('httplive/config', {
	defaultPort:'%s',
	savePath: '${ContextPath}/httplive/webcli/api/save',
	fetchPath: '${ContextPath}/httplive/webcli/api/endpoint',
	deletePath: '${ContextPath}/httplive/webcli/api/deleteendpoint',
	treePath: '${ContextPath}/httplive/webcli/api/tree',
	componentId: ''
});`, Environments.Ports)), Environments.ContextPath)
	c.Data(http.StatusOK, "application/javascript", fileContent)
	c.Abort()
}

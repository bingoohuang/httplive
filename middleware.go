package httplive

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/bingoohuang/httplive/internal/process"
	"github.com/bingoohuang/httplive/internal/res"
	"github.com/bingoohuang/httplive/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/mssola/user_agent"
	"github.com/sirupsen/logrus"
)

//go:embed public
var publicFS embed.FS

// StaticFileMiddleware ...
func StaticFileMiddleware(c *gin.Context) {
	p := trimContextPath(c)
	if util.HasPrefix(p, "/httplive/webcli", "/httplive/ws") {
		c.Next()
		return
	}

	uriPath := strings.TrimPrefix(p, "/httplive")
	assetPath := path.Join("/public", uriPath)
	if c.Request.Method == http.MethodGet && uriPath == "/" {
		assetPath = "/public/index.html"
	}

	if res.TryGetFile(publicFS, c, assetPath, Envs.ContextPath) {
		c.Abort()
		return
	}

	c.Next()
}

// APIMiddleware ...
func APIMiddleware(c *gin.Context) {
	p := trimContextPath(c)
	ua := user_agent.New(c.Request.UserAgent())
	isBrowser := ua.OS() != ""
	isBrowserIndex := isBrowser && c.Request.URL.Path == "/" && c.Query("_hl") == ""

	if isBrowserIndex || util.AnyOf(p, "/favicon.ico") || util.HasPrefix(p, "/httplive/") {
		c.Next()
		return
	}

	var bufferRead bytes.Buffer
	c.Request.Body = CreateTeeReader(c.Request.Body, &bufferRead)

	if result := serveAPI(c.Writer, c.Request); result.RouterServed {
		if broadcastThrottler.Allow() {
			broadcast(c, &bufferRead, result)
		}

		c.Abort()
		return
	}

	c.Next()
}

type ReadCloser struct {
	io.Reader
	io.Closer
}

func CreateTeeReader(rc io.ReadCloser, w io.Writer) io.ReadCloser {
	tee := io.TeeReader(rc, w)
	return &ReadCloser{Reader: tee, Closer: rc}
}

func broadcast(c *gin.Context, requestBody *bytes.Buffer, rr process.RouterResult) {
	msg := process.WsMessage{
		Time:   util.TimeFmt(time.Now()),
		Host:   c.Request.Host,
		Body:   util.GetRequestBody(requestBody),
		Method: c.Request.Method,
		Path:   c.Request.URL.Path,
		Query:  util.ConvertHeader(c.Request.URL.Query()),
		Header: util.GetHeaders(c),

		Response:       string(util.CompactJSON(rr.RouterBody)),
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
	if Envs.ContextPath != "/" {
		p = strings.TrimPrefix(p, Envs.ContextPath)
	}

	return util.Or(p, "/")
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
});`, Envs.Ports)), Envs.ContextPath)
	c.Data(http.StatusOK, "application/javascript", fileContent)
	c.Abort()
}

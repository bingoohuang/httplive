package httplive

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/bingoohuang/gou/ran"
	"github.com/skratchdot/open-golang/open"

	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// CreateEndpointKey ...
func CreateEndpointKey(method string, endpoint string) string {
	return strings.ToLower(method + endpoint)
}

// Broadcast ...
func Broadcast(c *gin.Context, rr routerResult) {
	msg := WsMessage{
		Time:           time.Now().Format("2006-01-02 15:04:05.000"),
		Host:           c.Request.Host,
		Body:           GetRequestBody(c),
		Method:         c.Request.Method,
		Path:           c.Request.URL.Path,
		Query:          convertHeader(c.Request.URL.Query()),
		Header:         GetHeaders(c),
		Response:       compactJSON(rr.RouterBody),
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

// GetHeaders ...
func GetHeaders(c *gin.Context) map[string]string {
	hdr := make(map[string]string, len(c.Request.Header))
	for k, v := range c.Request.Header {
		hdr[k] = v[0]
	}

	return hdr
}

// GetIP ...
func GetIP(c *gin.Context) string {
	ip := c.ClientIP()
	return ip
}

// GetMultiPartFormValue ...
func GetMultiPartFormValue(c *gin.Context) interface{} {
	_ = c.Request.ParseMultipartForm(32 * 1024 * 1024) // nolint gomnd 32M

	if c.Request.MultipartForm != nil {
		form := make(gin.H)
		for key, values := range c.Request.MultipartForm.Value {
			form[key] = strings.Join(values, "")
		}

		for key, file := range c.Request.MultipartForm.File {
			for k, f := range file {
				formKey := fmt.Sprintf("%s%d", key, k)
				form[formKey] = gin.H{"filename": f.Filename, "size": f.Size}
			}
		}

		return form
	}

	return nil
}

// GetFormBody ...
func GetFormBody(c *gin.Context) interface{} {
	_ = c.Request.ParseForm()

	form := make(map[string]string)
	for key, values := range c.Request.PostForm {
		form[key] = strings.Join(values, "")
	}

	return form
}

// TryBind ...
func TryBind(c *gin.Context) interface{} {
	var model interface{}
	err := c.Bind(&model)
	if err != nil {
		return nil
	}

	return model
}

// GetRequestBody ...
func GetRequestBody(c *gin.Context) interface{} {
	if c.Request.Method == http.MethodGet {
		return nil
	}

	multiPartFormValue := GetMultiPartFormValue(c)
	if multiPartFormValue != nil {
		return multiPartFormValue
	}

	formBody := GetFormBody(c)
	if formBody != nil {
		return formBody
	}

	switch c.ContentType() {
	case binding.MIMEJSON:
		return TryBind(c)
	default:
		body, _ := ioutil.ReadAll(c.Request.Body)
		return string(body)
	}
}

// IsJSONStr tests string s is in JSON format.
func IsJSONStr(s string) bool {
	return IsJSONBytes([]byte(s))
}

// IsJSONBytes tests bytes b is in JSON format.
func IsJSONBytes(b []byte) bool {
	if len(b) == 0 {
		return false
	}

	var m interface{}
	return json.Unmarshal(b, &m) == nil
}

// DetectContentType detects the contentType of b.
func DetectContentType(b []byte) string {
	if IsJSONBytes(b) {
		return "application/json; charset=utf-8"
	}

	return "text/plain; charset=utf-8"
}

// OpenExplorerWithContext ...
func OpenExplorerWithContext(contextPath, port string) {
	switch runtime.GOOS {
	case "windows", "darwin":
		if contextPath == "/" {
			contextPath = ""
		}

		_ = open.Run("http://127.0.0.1:" + port + contextPath + "?" + ran.String(10))
	}
}

// Throttle ...
type Throttle struct {
	tokenC chan bool
	stopC  chan bool
}

// MakeThrottle ...
func MakeThrottle(tokensNum int, duration time.Duration) *Throttle {
	t := &Throttle{
		tokenC: make(chan bool, tokensNum),
		stopC:  make(chan bool, 1),
	}

	go func() {
		for {
			select {
			case <-t.stopC:
				return
			default:
				t.putTokens(tokensNum)
				time.Sleep(duration)
			}
		}
	}()

	return t
}

func (t *Throttle) putTokens(tokensNum int) {
	for i := 0; i < tokensNum; i++ {
		select {
		case t.tokenC <- true:
		default:
			return
		}
	}
}

// Stop ...
func (t *Throttle) Stop() {
	t.stopC <- true
}

// Allow ...
func (t *Throttle) Allow() bool {
	select {
	case <-t.tokenC:
		return true
	default:
		return false
	}
}

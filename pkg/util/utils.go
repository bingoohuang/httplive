package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/bingoohuang/gou/ran"
	"github.com/skratchdot/open-golang/open"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// HasContentType determine whether the request `content-type` includes a
// server-acceptable mime-type
// Failure should yield an HTTP 415 (`http.StatusUnsupportedMediaType`)
func HasContentType(r *http.Request, mimetype string) bool {
	contentType := r.Header.Get("Content-type")
	if contentType == "" {
		return mimetype == "application/octet-stream"
	}

	for _, v := range strings.Split(contentType, ",") {
		if t, _, err := mime.ParseMediaType(v); err != nil {
			break
		} else if t == mimetype {
			return true
		}
	}

	return false
}

// JSON jsonifies the value.
func JSON(v interface{}) []byte {
	vv, _ := json.Marshal(v)
	return vv
}

// CompactJSON compact a json byte slice or wrap it to raw value.
func CompactJSON(b []byte) []byte {
	var out bytes.Buffer
	if err := json.Compact(&out, b); err != nil {
		v, _ := json.Marshal(map[string]string{"raw": string(b)})
		return v
	}

	return out.Bytes()
}

// ConvertHeader convert s head map[string][]string to map[string]string.
func ConvertHeader(query map[string][]string) map[string]string {
	q := make(map[string]string)
	for k, v := range query {
		q[k] = strings.Join(v, " ")
	}

	return q
}

// JoinLowerKeys ...
func JoinLowerKeys(s ...string) string {
	return strings.ToLower(strings.Join(s, ""))
}

// GetHeaders ...
func GetHeaders(c *gin.Context) map[string]string {
	hdr := make(map[string]string, len(c.Request.Header))
	for k, v := range c.Request.Header {
		hdr[k] = v[0]
	}

	return hdr
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

// IsJSONBytes tests bytes b is in JSON format.
func IsJSONBytes(b []byte) bool {
	if len(b) == 0 {
		return false
	}

	var m interface{}
	return json.Unmarshal(b, &m) == nil
}

const (
	ContentTypeText = "text/plain; charset=utf-8"
	ContentTypeJSON = "application/json; charset=utf-8"
)

// TimeFmt format time.
func TimeFmt(t time.Time) string {
	return t.Format("2006-01-02 15:04:05.0000")
}

// DetectContentType detects the contentType of b.
func DetectContentType(b []byte) string {
	if IsJSONBytes(b) {
		return ContentTypeJSON
	}

	return ContentTypeText
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

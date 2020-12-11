package process

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/bingoohuang/httplive/pkg/http2curl"
	"io/ioutil"
	"mime"
	"net/http"
	"time"

	"github.com/bingoohuang/gou/str"
	"github.com/bingoohuang/httplive/pkg/util"
	"github.com/gin-gonic/gin"
)

// ContextKey as context key type.
type ContextKey int

// RouterResultKey as RouterResult key
const RouterResultKey ContextKey = iota

// ID is the ID for UnmarshalJSON from integer.
type ID string

// UnmarshalJSON unmarshals JSON from integer or string.
func (i *ID) UnmarshalJSON(b []byte) error {
	*i = ID(b)

	return nil
}

// Int convert ID to integer.
func (i ID) Int() int { return str.ParseInt(string(i)) }

// APIDataModel ...
type APIDataModel struct {
	ID          ID     `json:"id" form:"id"`
	Endpoint    string `json:"endpoint" form:"endpoint"`
	Method      string `json:"method" form:"method"`
	MimeType    string `json:"mimeType"`
	Filename    string `json:"filename"`
	FileContent []byte `json:"-"`
	Body        string `json:"body"`

	dynamicValuers []DynamicValue
	ServeFn        gin.HandlerFunc `json:"-"`
}

// WsMessage ...
type WsMessage struct {
	Time           string            `json:"time"`
	Host           string            `json:"host"`
	Body           interface{}       `json:"body"`
	Response       json.RawMessage   `json:"response"`
	ResponseStatus int               `json:"status"`
	ResponseHeader map[string]string `json:"responseHeader"`
	ResponseSize   int               `json:"responseSize"`
	Header         map[string]string `json:"header"`
	Method         string            `json:"method"`
	Path           string            `json:"path"`
	Query          map[string]string `json:"query"`
	RemoteAddr     string            `json:"remoteAddr"`
}

// Endpoint is the structure for table httplive_endpoint.
type Endpoint struct {
	ID         ID     `name:"id"`
	Endpoint   string `name:"endpoint"`
	Methods    string `name:"methods"`
	MimeType   string `name:"mime_type"`
	Filename   string `name:"filename"`
	Body       string `name:"body"`
	CreateTime string `name:"create_time"`
	UpdateTime string `name:"update_time"`
	DeletedAt  string `name:"deleted_at"`
}

func (ep *APIDataModel) HandleFileDownload(c *gin.Context) {
	rr := c.Request.Context().Value(RouterResultKey).(RouterResult)
	rr.RouterServed = true
	rr.Filename = ep.Filename
	c.Status(http.StatusOK)

	if c.Query("_view") == "" {
		h := c.Header
		h("Content-Disposition", mime.FormatMediaType("attachment",
			map[string]string{"filename": ep.Filename}))
		h("Content-Description", "File Transfer")
		h("Content-Type", "application/octet-stream")
		h("Content-Transfer-Encoding", "binary")
		h("Expires", "0")
		h("Cache-Control", "must-revalidate")
		h("Pragma", "public")
	}

	http.ServeContent(c.Writer, c.Request, ep.Filename, time.Now(),
		bytes.NewReader(ep.FileContent))
}

// JsTreeDataModel ...
type JsTreeDataModel struct {
	ID        int               `json:"id"`
	Key       string            `json:"key"`
	OriginKey string            `json:"originKey"`
	Text      string            `json:"text"`
	Type      string            `json:"type"`
	Children  []JsTreeDataModel `json:"children"`
}

func (a APIDataModel) getLabelByMethod() string {
	switch a.Method {
	case http.MethodGet:
		return "label label-primary label-small"
	case http.MethodPost:
		return "label label-success label-small"
	case http.MethodPut:
		return "label label-warning label-small"
	case http.MethodDelete:
		return "label label-danger label-small"
	default:
		return "label label-default label-small"
	}
}

func (a APIDataModel) CreateJsTreeModel() JsTreeDataModel {
	model := JsTreeDataModel{
		ID:        a.ID.Int(),
		OriginKey: util.JoinLowerKeys(a.Method, a.Endpoint),
		Key:       a.Endpoint,
		Text:      a.Endpoint,
		Children:  []JsTreeDataModel{},
	}

	model.Type = a.Method
	model.Text = fmt.Sprintf(`<span class="%v">%v</span> %v`, a.getLabelByMethod(), a.Method, a.Endpoint)

	return model
}

func (ep APIDataModel) HandleJSON(c *gin.Context) {
	if viewProcess(c, ep) || ep.ServeFn == nil {
		return
	}

	cw := util.NewGinCopyWriter(c.Writer)
	c.Writer = cw
	ep.ServeFn(c)

	rr := c.Request.Context().Value(RouterResultKey).(*RouterResult)
	if !rr.RouterServed {
		rr.RouterServed = true
		rr.RouterBody = cw.Bytes()
	}

	rr.RemoteAddr = c.Request.RemoteAddr
	rr.ResponseSize = cw.Size()
	rr.ResponseStatus = cw.Status()
	rr.ResponseHeader = util.ConvertHeader(cw.Header())
}

func viewProcess(c *gin.Context, ep APIDataModel) bool {
	switch c.Query("_view") {
	case "curl":
		values := c.Request.URL.Query()
		delete(values, "_view")
		c.Request.URL.RawQuery = values.Encode()
		cmd, _ := http2curl.GetCurlCmd(c.Request)
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(cmd.String()))
	case "conf":
		body := []byte(ep.Body)
		c.Data(http.StatusOK, util.DetectContentType(body), body)
	default:
		return false
	}

	return true
}

func dynamicProcess(c *gin.Context, ep APIDataModel) bool {
	if len(ep.dynamicValuers) == 0 {
		return false
	}

	reqBody, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		fmt.Println(err)
		return false
	}

	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody))

	for _, v := range ep.dynamicValuers {
		parameters := make(gin.H, len(v.ParametersEvaluator))
		for k, valuer := range v.ParametersEvaluator {
			parameters[k] = valuer(reqBody, c)
		}

		evaluateResult, err := v.Expr.Evaluate(parameters)
		if err != nil {
			fmt.Println(err)
			return false
		}

		if yes, ok := evaluateResult.(bool); ok && yes {
			v.responseDynamic(c)

			return true
		}
	}

	return false
}

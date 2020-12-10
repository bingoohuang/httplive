package httplive

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/bingoohuang/gou/str"
	"github.com/gorilla/websocket"
)

// EnvVars ...
type EnvVars struct {
	WorkingDir  string
	DBFile      string
	DBFullPath  string
	Ports       string // Hosting ports, eg. 5003,5004.
	ContextPath string
}

// nolint gochecknoglobals
var (
	// Environments ...
	Environments = EnvVars{}

	// Clients ...
	Clients = make(map[string]*websocket.Conn)
)

// Init initializes the environments.
func (r *EnvVars) Init() {
	if strings.HasSuffix(r.ContextPath, "/") {
		r.ContextPath = r.ContextPath[:len(r.ContextPath)-1]
	}

	if !HasPrefix(r.ContextPath, "/") {
		r.ContextPath = "/" + r.ContextPath
	}
}

// WebCliController ...
type WebCliController struct{}

// ID is the ID for UnmarshalJSON from integer.
type ID string

// UnmarshalJSON unmarshals JSON from integer or string.
func (i *ID) UnmarshalJSON(b []byte) error {
	*i = ID(b)

	return nil
}

// Int convert ID to integer.
func (i ID) Int() int { return str.ParseInt(string(i)) }

type valuer func(reqBody []byte, c *gin.Context) interface{}

// APIDataModel ...
type APIDataModel struct {
	ID          ID     `json:"id" form:"id"`
	Endpoint    string `json:"endpoint" form:"endpoint"`
	Method      string `json:"method" form:"method"`
	MimeType    string `json:"mimeType"`
	Filename    string `json:"filename"`
	FileContent []byte `json:"-"`
	Body        string `json:"body"`

	dynamicValuers []dynamicValue
	serveFn        gin.HandlerFunc
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

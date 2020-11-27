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

	if !strings.HasPrefix(r.ContextPath, "/") {
		r.ContextPath = "/" + r.ContextPath
	}
}

// IPResponse ...
type IPResponse struct {
	Origin string `json:"origin"`
}

// UserAgentResponse ...
type UserAgentResponse struct {
	UserAgent string `json:"user-agent"`
}

// HeadersResponse ...
type HeadersResponse struct {
	Headers map[string]string `json:"headers"`
}

// CookiesResponse ...
type CookiesResponse struct {
	Cookies map[string]string `json:"cookies"`
}

// JSONResponse ...
type JSONResponse interface{}

// GetResponse ...
type GetResponse struct {
	Args map[string][]string `json:"args"`
	HeadersResponse
	IPResponse
	URL string `json:"url"`
}

// PostResponse ...
type PostResponse struct {
	Args map[string][]string `json:"args"`
	Data JSONResponse        `json:"data"`
	Form map[string]string   `json:"form"`
	HeadersResponse
	IPResponse
	URL string `json:"url"`
}

// GzipResponse ...
type GzipResponse struct {
	HeadersResponse
	IPResponse
	Gzipped bool `json:"gzipped"`
}

// DeflateResponse ...
type DeflateResponse struct {
	HeadersResponse
	IPResponse
	Deflated bool `json:"deflated"`
}

// BasicAuthResponse ...
type BasicAuthResponse struct {
	Authenticated bool   `json:"authenticated"`
	User          string `json:"string"`
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
func (i ID) Int() int {
	return str.ParseInt(string(i))
}

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

// Pair ...
type Pair struct {
	Key   string
	Value APIDataModel
}

// PairList ...
type PairList []Pair

func (p PairList) Len() int           { return len(p) }
func (p PairList) Less(i, j int) bool { return p[i].Value.ID > p[j].Value.ID }
func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

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
	Time     string              `json:"time"`
	Host     string              `json:"host"`
	Body     interface{}         `json:"body"`
	Response json.RawMessage     `json:"response"`
	Header   map[string]string   `json:"header"`
	Method   string              `json:"method"`
	Path     string              `json:"path"`
	Query    map[string][]string `json:"query"`
}

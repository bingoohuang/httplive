package httplive

import (
	"strings"

	"github.com/bingoohuang/httplive/pkg/util"

	"github.com/gorilla/websocket"
)

// EnvVars ...
type EnvVars struct {
	DBFile      string
	DBFullPath  string
	Ports       string // Hosting ports, eg. 5003,5004.
	ContextPath string
	Logging     bool
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

	if !util.HasPrefix(r.ContextPath, "/") {
		r.ContextPath = "/" + r.ContextPath
	}
}

// WebCliController ...
type WebCliController struct{}

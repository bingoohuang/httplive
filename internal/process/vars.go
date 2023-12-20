package process

import (
	"encoding/base64"
	"strings"

	"github.com/bingoohuang/httplive/pkg/util"
)

// EnvVars ...
type EnvVars struct {
	DBFile      string
	DBFullPath  string
	Ports       string // Hosting ports, eg. 5003,5004.
	ContextPath string
	CaRoot      string
	BasicAuth   string
	HTTPretty   bool
}

// Init initializes the environments.
func (r *EnvVars) Init() {
	r.ContextPath = strings.TrimPrefix(r.ContextPath, "/")

	if !util.HasPrefix(r.ContextPath, "/") {
		r.ContextPath = "/" + r.ContextPath
	}

	if r.BasicAuth != "" {
		r.BasicAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(r.BasicAuth))
	}

	Envs = r
}

var Envs *EnvVars

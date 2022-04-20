package httplive

import (
	"github.com/bingoohuang/httplive/internal/process"
	"github.com/gorilla/websocket"
)

var (
	// Envs ...
	Envs = process.EnvVars{}

	// Clients ...
	Clients = make(map[string]*websocket.Conn)
)

// WebCliController ...
type WebCliController struct{}

package httplive

import "github.com/gorilla/websocket"

// nolint gochecknoglobals
var (
	// Environments ...
	Environments = EnvVars{}

	// Clients ...
	Clients = make(map[string]*websocket.Conn)
)

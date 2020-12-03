package main

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/bingoohuang/gor/giu"

	"github.com/bingoohuang/httplive"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/urfave/cli"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	app := cli.NewApp()
	env := &httplive.Environments

	app.Name = "httplive"
	app.Usage = "HTTP Request & Response Service, Mock HTTP"
	app.Version = httplive.Version + " @ " + httplive.UpdateTime
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "ports, p",
			Value:       "5003",
			Usage:       "Hosting ports, eg. 5003,5004.",
			Destination: &env.Ports,
		},
		cli.StringFlag{
			Name:        "dbpath, d",
			Value:       "",
			Usage:       "Full path of the httplive.db.",
			Destination: &env.DBFullPath,
		},
		cli.StringFlag{
			Name:        "contextpath, c",
			Value:       "",
			Usage:       "context path of httplive service",
			Destination: &env.ContextPath,
		},
	}

	app.Action = func(c *cli.Context) error { return host(env) }

	_ = app.Run(os.Args)
}

func createDB(env *httplive.EnvVars) error {
	fullPath, createDbRequired := fixDBPath(env)

	env.DBFile = fullPath

	return httplive.CreateDB(createDbRequired)
}

func fixDBPath(env *httplive.EnvVars) (string, bool) {
	fullPath := env.DBFullPath
	if fullPath == "" {
		return path.Join(env.WorkingDir, "httplive.db"), true
	}

	if s, err := os.Stat(fullPath); err == nil {
		if s.IsDir() {
			return path.Join(fullPath, "httplive.db"), true
		}

		return fullPath, false
	}

	p := fullPath
	if strings.HasSuffix(fullPath, ".db") {
		p = filepath.Dir(p)
	} else {
		fullPath = path.Join(p, "httplive.db")
	}

	if _, err := os.Stat(p); os.IsNotExist(err) {
		err := os.MkdirAll(p, 0o644)
		if err != nil {
			logrus.Fatalf("create  dir %s error %v", fullPath, err)
		}
	}

	return fullPath, true
}

func host(env *httplive.EnvVars) error {
	env.Init()
	portsArr := strings.Split(env.Ports, ",")

	env.WorkingDir, _ = os.Getwd()
	if err := createDB(env); err != nil {
		logrus.Warnf("failed to create DB %v", err)
		return err
	}

	r := gin.New()
	r.Use(httplive.APIMiddleware, httplive.StaticFileMiddleware,
		httplive.CORSMiddleware, httplive.ConfigJsMiddleware)

	r.GET(httplive.JoinContextPath("/httplive/ws"), wshandler)

	ga := giu.NewAdaptor()
	gw := ga.Route(r.Group(httplive.JoinContextPath("/httplive/webcli")))
	gw.HandleFn(new(httplive.WebCliController))

	for _, p := range portsArr {
		go func(port string) {
			if err := r.Run(":" + port); err != nil {
				panic(err)
			}
		}(p)
	}

	httplive.OpenExplorerWithContext(env.ContextPath, portsArr[0])

	select {}
}

// nolint gochecknoglobals
var wsupgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024, // nolint gomnd
	WriteBufferSize: 1024, // nolint gomnd
}

func wshandler(c *gin.Context) {
	connID := c.Request.URL.Query().Get("connectionId")
	if connID != "" {
		if conn := httplive.Clients[connID]; conn != nil {
			return
		}
	}

	conn, err := wsupgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logrus.Warnf("Failed to set websocket upgrade: %+v", err)
		return
	}

	httplive.Clients[connID] = conn

	for {
		t, msg, err := conn.ReadMessage()
		if err != nil {
			delete(httplive.Clients, connID)
			break
		}

		_ = conn.WriteMessage(t, msg)
	}
}

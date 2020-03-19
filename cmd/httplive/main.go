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
	//gin.SetMode(gin.ReleaseMode)
	app := cli.NewApp()
	env := &httplive.Environments

	app.Name = "httplive"
	app.Usage = "HTTP Request & Response Service, Mock HTTP"
	app.Version = "0.0.1"
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
	}

	app.Action = func(c *cli.Context) error { return host(env) }

	_ = app.Run(os.Args)
}

func createDB(env *httplive.EnvVars) error {
	fullPath := httplive.Environments.DBFullPath
	if fullPath == "" {
		fullPath = path.Join(env.WorkingDir, "httplive.db")
	} else {
		p := filepath.Dir(fullPath)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			logrus.Fatalf("fullPath %s error %v", fullPath, err)
		}
	}

	env.DBFile = fullPath

	return httplive.CreateDBBucket()
}

func host(env *httplive.EnvVars) error {
	portsArr := strings.Split(env.Ports, ",")

	env.WorkingDir, _ = os.Getwd()
	env.DefaultPort = portsArr[0]

	_ = createDB(env)

	httplive.InitDBValues()

	r := gin.Default()
	r.Use(httplive.APIMiddleware(), httplive.StaticFileMiddleware(),
		httplive.CORSMiddleware(), httplive.ConfigJsMiddleware())
	r.GET("/ws", wshandler)

	ga := giu.NewAdaptor()
	gw := ga.Route(r.Group("/webcli"))
	gw.HandleFn(new(httplive.WebCliController))

	r.NoRoute(func(c *gin.Context) {
		httplive.Broadcast(c)
		c.Status(http.StatusNotFound)
		c.File("./public/404.html")
	})

	for _, p := range portsArr {
		go func(port string) {
			_ = r.Run(":" + port)
		}(p)
	}

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

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

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

	app.Action = func(c *cli.Context) error {
		host(env)
		return nil
	}

	_ = app.Run(os.Args)
}

func createDB(env *httplive.EnvVars) error {
	if fullPath := httplive.Environments.DBFullPath; fullPath != "" {
		p := filepath.Dir(fullPath)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			log.Fatal(err)
		}

		env.DBFile = fullPath
	} else {
		env.DBFile = path.Join(env.WorkingDir, "httplive.db")
	}

	return httplive.CreateDBBucket()
}

func host(env *httplive.EnvVars) {
	portsArr := strings.Split(env.Ports, ",")

	env.WorkingDir, _ = os.Getwd()
	env.DefaultPort = portsArr[0]

	_ = createDB(env)

	httplive.InitDBValues()

	r := gin.Default()

	r.Use(httplive.StaticFileMiddleware())

	r.GET("/ws", func(c *gin.Context) {
		wshandler(c.Writer, c.Request)
	})

	r.Use(httplive.CORSMiddleware(), httplive.ConfigJsMiddleware())

	ga := giu.NewAdaptor()

	webcli := r.Group("/webcli")
	gw := ga.Route(webcli)

	ctrl := new(httplive.WebCliController)
	gw.HandleFn(ctrl)

	r.Use(httplive.APIMiddleware())

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
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024, // nolint gomnd
	WriteBufferSize: 1024, // nolint gomnd
}

func wshandler(w http.ResponseWriter, r *http.Request) {
	connID := r.URL.Query().Get("connectionId")
	if connID != "" {
		conn := httplive.Clients[connID]
		if conn != nil {
			return
		}
	}

	conn, err := wsupgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Failed to set websocket upgrade: %+v", err)
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

package main

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bingoohuang/httplive/pkg/util"

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

	app.Action = func(c *cli.Context) error {
		if c.NArg() > 0 {
			fmt.Println("Unknown args:", c.Args())
			// cli.ShowAppHelp(c)
			os.Exit(1)
		}

		return host(env)
	}

	_ = app.Run(os.Args)
}

func createDB(env *httplive.EnvVars) error {
	fullPath, createDbRequired := fixDBPath(env)

	env.DBFile = fullPath

	return httplive.CreateDB(createDbRequired)
}

const defaultDb = "httplive.db"

func fixDBPath(env *httplive.EnvVars) (string, bool) {
	fullPath := env.DBFullPath
	workingDir, _ := os.Getwd()
	if fullPath == "" {
		return path.Join(workingDir, defaultDb), true
	}

	if s, err := os.Stat(fullPath); err == nil {
		if s.IsDir() {
			return path.Join(fullPath, defaultDb), true
		}

		return fullPath, false
	}

	p := fullPath
	if strings.HasSuffix(fullPath, ".db") {
		p = filepath.Dir(p)
	} else {
		fullPath = path.Join(p, defaultDb)
	}

	if _, err := os.Stat(p); os.IsNotExist(err) {
		if err := os.MkdirAll(p, 0644); err != nil {
			logrus.Fatalf("create  dir %s error %v", fullPath, err)
		}
	}

	return fullPath, true
}

func host(env *httplive.EnvVars) error {
	env.Init()
	portsArr := strings.Split(env.Ports, ",")

	if err := createDB(env); err != nil {
		logrus.Warnf("failed to create DB %v", err)
		return err
	}

	r := gin.New()
	r.Use(httplive.APIMiddleware, httplive.StaticFileMiddleware,
		util.CORSMiddleware, httplive.ConfigJsMiddleware)

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

	go util.OpenExplorerWithContext(env.ContextPath, portsArr[0])

	select {}
}

// nolint gochecknoglobals
var wsupgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024, // nolint gomnd
	WriteBufferSize: 1024, // nolint gomnd
}

func wshandler(c *gin.Context) {
	connID := c.Query("connectionId")
	if httplive.Clients[connID] != nil {
		return
	}

	conn, err := wsupgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logrus.Warnf("Failed to set websocket upgrade: %+v", err)
		return
	}

	httplive.Clients[connID] = conn

	for {
		if t, msg, err := conn.ReadMessage(); err != nil {
			delete(httplive.Clients, connID)
			break
		} else {
			_ = conn.WriteMessage(t, msg)
		}
	}
}

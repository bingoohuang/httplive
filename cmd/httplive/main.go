package main

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bingoohuang/gg/pkg/sigx"
	"github.com/bingoohuang/gg/pkg/ss"

	"github.com/bingoohuang/golog"
	"github.com/bingoohuang/gor/giu"
	"github.com/bingoohuang/httplive"
	"github.com/bingoohuang/httplive/internal/process"
	"github.com/bingoohuang/httplive/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	app := cli.NewApp()
	env := &httplive.Environments

	app.Name = "httplive"
	app.Usage = "HTTP Request & Response Service, Mock HTTP"
	app.Version = httplive.Version()
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "ports, p", Value: "5003", Usage: "Hosting ports, eg. 5003,5004", Destination: &env.Ports},
		cli.StringFlag{Name: "dbpath, d", Value: "", Usage: "Full path of the httplive.db", Destination: &env.DBFullPath},
		cli.StringFlag{Name: "context, c", Value: "", Usage: "context path of httplive service", Destination: &env.ContextPath},
		cli.BoolFlag{Name: "log, l", Usage: "enable golog logging", Destination: &env.Logging},
	}

	app.Action = func(c *cli.Context) error {
		if c.NArg() > 0 {
			fmt.Println("Unknown args:", c.Args())
			os.Exit(1)
		}

		return host(env)
	}

	sigx.RegisterSignalProfile(nil)
	_ = app.Run(os.Args)
}

func createDB(env *httplive.EnvVars) error {
	fullPath := fixDBPath(env)
	env.DBFile = fullPath
	return httplive.CreateDB()
}

const defaultDb = "httplive.bolt"

func fixDBPath(env *httplive.EnvVars) string {
	fullPath := env.DBFullPath
	workingDir, _ := os.Getwd()
	if fullPath == "" {
		return path.Join(workingDir, defaultDb)
	}

	if s, err := os.Stat(fullPath); err == nil {
		if s.IsDir() {
			return path.Join(fullPath, defaultDb)
		}

		return fullPath
	}

	p := fullPath
	if ss.HasSuffix(fullPath, ".bolt", ".db") {
		p = filepath.Dir(p)
	} else {
		fullPath = path.Join(p, defaultDb)
	}

	if _, err := os.Stat(p); os.IsNotExist(err) {
		if err := os.MkdirAll(p, 0o644); err != nil {
			logrus.Fatalf("create  dir %s error %v", fullPath, err)
		}
	}

	return fullPath
}

func host(env *httplive.EnvVars) error {
	env.Init()

	if err := createDB(env); err != nil {
		logrus.Warnf("failed to create DB %v", err)
		return err
	}

	if env.Logging {
		golog.Setup(golog.Spec("stdout"))
	} else {
		golog.DisableLogging()
	}

	r := gin.New()
	r.Use(httplive.APIMiddleware, httplive.StaticFileMiddleware,
		util.CORSMiddleware, httplive.ConfigJsMiddleware)

	r.GET(httplive.JoinContextPath("/httplive/ws"), wshandler)

	ga := giu.NewAdaptor()
	group := r.Group(httplive.JoinContextPath("/httplive/webcli"))
	group.Use(process.AdminAuth)
	ga.Route(group).HandleFn(new(httplive.WebCliController))

	portsArr := strings.Split(env.Ports, ",")
	for _, p := range portsArr {
		go func(port string) {
			if err := r.Run(":" + port); err != nil {
				fmt.Println(err)
			}
		}(p)
	}

	go util.OpenExplorerWithContext(env.ContextPath, portsArr[0])

	select {}
}

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

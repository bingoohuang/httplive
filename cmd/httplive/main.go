package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/bingoohuang/httplive/pkg/gzip"

	"github.com/bingoohuang/gg/pkg/netx"
	"github.com/bingoohuang/golog"

	"github.com/bingoohuang/gg/pkg/ctl"
	"github.com/bingoohuang/gg/pkg/fla9"

	"github.com/bingoohuang/gg/pkg/sigx"
	"github.com/bingoohuang/gg/pkg/ss"

	"github.com/bingoohuang/gor/giu"
	"github.com/bingoohuang/httplive"
	"github.com/bingoohuang/httplive/internal/process"
	"github.com/bingoohuang/httplive/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	env := &httplive.Envs

	f := fla9.NewFlagSet(os.Args[0]+" (HTTP Request & Response Service, Mock HTTP)", fla9.ExitOnError)
	f.StringVar(&env.BasicAuth, "basic,b", "", "basic auth, format user:pass")
	f.StringVar(&env.Ports, "ports,p", "5003", "Hosting ports, eg. 5003,5004:https")
	f.StringVar(&env.DBFullPath, "dbpath,d", "", "Full path of the httplive.bolt")
	f.StringVar(&env.ContextPath, "context,c", "", "Context path of httplive http service")
	f.StringVar(&env.CaRoot, "ca", ".cert", "Cert root path of localhost.key and localhost.pem")
	f.BoolVar(&env.Logging, "log,l", false, "Enable golog logging")
	pInit := f.Bool("init", false, "Create initial ctl and exit")
	pVersion := f.Bool("version,v", false, "Create initial ctl and exit")
	_ = f.Parse(os.Args[1:])
	ctl.Config{Initing: *pInit, PrintVersion: *pVersion}.ProcessInit()

	if env.Logging {
		golog.Setup(golog.Spec("stdout"))
	} else {
		golog.DisableLogging()
	}

	sigx.RegisterSignalProfile()

	if f.NArg() > 0 {
		fmt.Println("Unknown args:", f.Args())
		os.Exit(1)
	}

	host(env)
}

func mkdirCerts(env *process.EnvVars) *netx.CertFiles {
	return netx.LoadCerts(env.CaRoot)
}

func createDB(env *process.EnvVars) error {
	fullPath := fixDBPath(env)
	env.DBFile = fullPath
	return httplive.CreateDB()
}

const defaultDb = "httplive.bolt"

func fixDBPath(env *process.EnvVars) string {
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
		p = path.Dir(p)
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

func host(env *process.EnvVars) {
	env.Init()

	if err := createDB(env); err != nil {
		logrus.Warnf("failed to create DB %v", err)
		return
	}

	r := gin.New()
	r.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithDecompressFn(gzip.DefaultDecompressHandle)))
	r.Use(httplive.APIMiddleware, httplive.StaticFileMiddleware,
		util.CORSMiddleware, httplive.ConfigJsMiddleware)

	wsPath := httplive.JoinContextPath("/httplive/ws", nil)
	r.GET(wsPath, wshandler)

	ga := giu.NewAdaptor()
	groupPath := httplive.JoinContextPath("/httplive/webcli", nil)
	group := r.Group(groupPath)
	group.Use(process.AdminAuth)
	ga.Route(group).HandleFn(new(httplive.WebCliController))
	var certFiles *netx.CertFiles

	portsArr := strings.Split(env.Ports, ",")
	for _, p := range portsArr {
		if strings.HasSuffix(p, ":https") {
			certFiles = mkdirCerts(env)
		}
	}

	for i, p := range portsArr {
		go func(seq int, port string) {
			tls := strings.HasSuffix(port, ":https")
			if tls {
				port = strings.TrimSuffix(port, ":https")
			}
			if seq == 0 {
				go util.OpenExplorer(tls, ss.ParseInt(port), env.ContextPath)
			}

			var err error
			if tls {
				log.Printf("Listening on %s for https", port)
				err = r.RunTLS(":"+port, certFiles.Cert, certFiles.Key)
			} else {
				log.Printf("Listening on %s for http", port)
				err = r.Run(":" + port)
			}
			if err != nil {
				log.Panicf("run on port %s failed: %v", port, err)
			}

		}(i, p)
	}

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

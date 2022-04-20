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

	fla := fla9.NewFlagSet(os.Args[0]+" (HTTP Request & Response Service, Mock HTTP)", fla9.ExitOnError)
	fla.StringVar(&env.BasicAuth, "basic,b", "", "basic auth, format user:pass")
	fla.StringVar(&env.Ports, "ports,p", "5003", "Hosting ports, eg. 5003,5004")
	fla.StringVar(&env.DBFullPath, "dbpath,d", "", "Full path of the httplive.bolt")
	fla.StringVar(&env.ContextPath, "context,c", "", "Context path of httplive http service")
	fla.StringVar(&env.CaRoot, "ca", ".cert", "Cert root path of x.key and x.pem")
	fla.BoolVar(&env.EnableHTTPS, "enableHttps,k", false, "Enable tls cert, using http other than https")
	fla.BoolVar(&env.Logging, "log,l", false, "Enable golog logging")
	pInit := fla.Bool("init", false, "Create initial ctl and exit")
	pVersion := fla.Bool("version,v", false, "Create initial ctl and exit")
	_ = fla.Parse(os.Args[1:])
	ctl.Config{Initing: *pInit, PrintVersion: *pVersion}.ProcessInit()

	if env.Logging {
		golog.Setup(golog.Spec("stdout"))
	} else {
		golog.DisableLogging()
	}

	certFile := mkdirCerts(env)

	sigx.RegisterSignalProfile()

	if fla.NArg() > 0 {
		fmt.Println("Unknown args:", fla.Args())
		os.Exit(1)
	}

	host(env, certFile)
}

func mkdirCerts(env *process.EnvVars) *netx.CertFiles {
	if !env.EnableHTTPS {
		return nil
	}

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

func host(env *process.EnvVars, certFiles *netx.CertFiles) {
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

	portsArr := strings.Split(env.Ports, ",")
	for _, p := range portsArr {
		go func(port string) {
			var err error
			if certFiles != nil {
				err = r.RunTLS(":"+port, certFiles.Cert, certFiles.Key)
			} else {
				err = r.Run(":" + port)
			}
			if err != nil {
				log.Panicf("run on port %s failed: %v", port, err)
			}
		}(p)
	}

	go util.OpenExplorer(certFiles != nil, ss.ParseInt(portsArr[0]), env.ContextPath)

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

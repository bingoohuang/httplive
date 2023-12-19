package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bingoohuang/fproxy"
	"github.com/bingoohuang/gg/pkg/ctl"
	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/netx"
	"github.com/bingoohuang/gg/pkg/osx/env"
	"github.com/bingoohuang/gg/pkg/sigx"
	"github.com/bingoohuang/gg/pkg/ss"
	_ "github.com/bingoohuang/godaemon/autoload"
	"github.com/bingoohuang/golog"
	"github.com/bingoohuang/gor/giu"
	"github.com/bingoohuang/httplive"
	"github.com/bingoohuang/httplive/internal/process"
	"github.com/bingoohuang/httplive/pkg/gzip"
	"github.com/bingoohuang/httplive/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	conf := &httplive.Envs

	f := fla9.NewFlagSet(os.Args[0]+" (HTTP Request & Response Service, Mock HTTP, env: GOLOG=0 )", fla9.ExitOnError)
	f.StringVar(&conf.BasicAuth, "basic,b", "", "basic auth, format user:pass")
	f.StringVar(&conf.Ports, "port,p", "5003", "Hosting ports, eg. 5003,5004:https,unix:$TMPDIR/a.sock")
	f.StringVar(&conf.DBFullPath, "dbpath,c", "", "Full path of the httplive.bolt")
	f.StringVar(&conf.ContextPath, "context", "", "Context path of httplive http service")
	f.StringVar(&conf.CaRoot, "ca", ".cert", "Cert root path of localhost.key and localhost.pem")
	pInit := f.Bool("init", false, "Create initial ctl and exit")
	pVersion := f.Bool("version,v", false, "Create initial ctl and exit")
	_ = f.Parse(os.Args[1:])
	ctl.Config{Initing: *pInit, PrintVersion: *pVersion}.ProcessInit()

	if env.Bool("GOLOG", true) {
		golog.Setup()
	} else {
		golog.DisableLogging()
	}

	sigx.RegisterSignalProfile()

	if f.NArg() > 0 {
		fmt.Println("Unknown args:", f.Args())
		os.Exit(1)
	}

	host(conf)
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
		if strings.HasPrefix(p, "unix:") {
		} else if !strings.HasSuffix(p, ":http") {
			certFiles = mkdirCerts(env)
		}
	}

	srv := &http.Server{Handler: r.Handler()}

	// Cleanup the sockfile.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatal("Server Shutdown:", err)
		}
		log.Println("Server exiting")
	}()

	var wg sync.WaitGroup
	for i, p := range portsArr {
		wg.Add(1)
		go func(seq int, port string) {
			defer wg.Done()
			serve(srv, seq, port, env, certFiles)
		}(i, p)
	}

	wg.Wait()
}

func TrimSuffix(s, suffix string) (string, bool) {
	if strings.HasSuffix(s, suffix) {
		return strings.TrimSuffix(s, suffix), true
	}

	return s, false
}

func serve(srv *http.Server, seq int, port string, env *process.EnvVars, certFiles *netx.CertFiles) {
	port, onlyHTTP := TrimSuffix(port, ":http")
	port, onlyTLS := TrimSuffix(port, ":https")
	unixSocket := ""
	if strings.HasPrefix(port, "unix:") {
		unixSocket = filepath.Clean(port[len("unix:"):])
	}
	go func() {
		if unixSocket != "" {
			os.Remove(unixSocket)
		}
	}()

	if seq == 0 && unixSocket != "" {
		go util.OpenExplorer(onlyTLS, ss.ParseInt(port), env.ContextPath)
	}

	var l net.Listener
	var err error
	for err == nil || errors.Is(err, io.EOF) {
		switch {
		case unixSocket != "":
			// Create a Unix domain socket and listen for incoming connections.
			l, err = net.Listen("unix", unixSocket)
			if err != nil {
				log.Panicln(err)
			}
			log.Printf("listen on socket: %s", unixSocket)
			err = srv.Serve(l)
		case onlyTLS:
			log.Printf("Listening on %s for https", port)
			srv.Addr = ":" + port
			err = srv.ListenAndServeTLS(certFiles.Cert, certFiles.Key)
		case onlyHTTP:
			log.Printf("Listening on %s for http", port)
			srv.Addr = ":" + port
			err = srv.ListenAndServe()
		default:
			log.Printf("Listening on %s for http and https", port)
			l, err = fproxy.CreateTLSListener(":"+port, certFiles.Cert, certFiles.Key)
			if err == nil {
				err = srv.Serve(l)
			}
		}

		if err != nil {
			log.Printf("server %s error: %v", port, err)
		}
	}
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

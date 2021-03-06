package httplive

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bingoohuang/httplive/pkg/http2curl"
	"github.com/bingoohuang/sysinfo"
	"github.com/mssola/user_agent"

	"github.com/bingoohuang/golog/pkg/hlog"

	"github.com/bingoohuang/sariaf"

	"github.com/bingoohuang/httplive/internal/process"
	"github.com/bingoohuang/httplive/pkg/util"

	"github.com/gin-gonic/gin"

	"github.com/bingoohuang/sqlx"
	_ "github.com/mattn/go-sqlite3" // import sqlite3
)

// Dao defines the api to access the database.
type Dao struct {
	CreateTable     func()
	ListEndpoints   func() []process.Endpoint
	FindEndpoint    func(ID process.ID) *process.Endpoint
	FindByEndpoint  func(endpoint string) *process.Endpoint
	AddEndpoint     func(process.Endpoint) int
	LastInsertRowID func() process.ID
	AddEndpointID   func(process.Endpoint)
	UpdateEndpoint  func(process.Endpoint)
	DeleteEndpoint  func(process.Endpoint)
	Logger          sqlx.DaoLogger
}

// CreateDao creates a dao.
func CreateDao(db *sql.DB) (*Dao, error) {
	dao := &Dao{Logger: &sqlx.DaoLogrus{}}
	err := sqlx.CreateDao(dao, sqlx.WithDB(db), sqlx.WithSQLStr(asset("httplive.sql")))

	return dao, err
}

//go:embed assets
var assetsFS embed.FS
var subAssets fs.FS

func init() {
	subAssets, _ = fs.Sub(assetsFS, "assets")
}

func asset(name string) string {
	data, err := fs.ReadFile(subAssets, name)
	if err != nil {
		panic(err)
	}

	return string(data)
}

// nolint gochecknoglobals
var dbLock sync.Mutex

// DBDo executes the f.
func DBDo(f func(dao *Dao) error) error {
	dbLock.Lock()
	defer dbLock.Unlock()

	db, err := sql.Open("sqlite3", Environments.DBFile)
	if err != nil {
		return err
	}

	defer db.Close()

	dao, err := CreateDao(db)
	if err != nil {
		return err
	}

	return f(dao)
}

// CreateDB ...
func CreateDB(createDbRequired bool) error {
	if createDbRequired {
		if err := DBDo(createDB); err != nil {
			return err
		}
	}

	SyncAPIRouter()

	return nil
}

func createDB(dao *Dao) error {
	dao.CreateTable()

	demo := dao.FindEndpoint("0")
	if demo != nil {
		return nil
	}

	now := util.TimeFmt(time.Now())
	i := -1
	f := func() process.ID {
		i++
		return process.ID(fmt.Sprintf("%d", i))
	}

	dao.AddEndpointID(process.Endpoint{
		ID: f(), Endpoint: "/api/demo", Methods: http.MethodGet, MimeType: "", Filename: "",
		Body: asset("apidemo.json"), CreateTime: now, UpdateTime: now, DeletedAt: "",
	})
	dao.AddEndpointID(process.Endpoint{
		ID: f(), Endpoint: "/dynamic/demo", Methods: http.MethodPost, MimeType: "", Filename: "",
		Body: asset("dynamicdemo.json"), CreateTime: now, UpdateTime: now, DeletedAt: "",
	})
	dao.AddEndpointID(process.Endpoint{
		ID: f(), Endpoint: "/proxy/demo", Methods: http.MethodGet, MimeType: "", Filename: "",
		Body: asset("proxydemo.json"), CreateTime: now, UpdateTime: now, DeletedAt: "",
	})
	dao.AddEndpointID(process.Endpoint{
		ID: f(), Endpoint: "/echo/:id", Methods: "ANY", MimeType: "", Filename: "",
		Body: asset("echo.json"), CreateTime: now, UpdateTime: now, DeletedAt: "",
	})
	dao.AddEndpointID(process.Endpoint{
		ID: f(), Endpoint: "/mockbin", Methods: "ANY", MimeType: "", Filename: "",
		Body: asset("mockbin.json"), CreateTime: now, UpdateTime: now, DeletedAt: "",
	})
	dao.AddEndpointID(process.Endpoint{
		ID: f(), Endpoint: "/eval", Methods: "ANY", MimeType: "", Filename: "",
		Body: asset("evaldemo.json"), CreateTime: now, UpdateTime: now, DeletedAt: "",
	})
	//dao.AddEndpointID(process.Endpoint{
	//	ID: f(), Endpoint: "/_internal/apiacl", Methods: "ANY", MimeType: "", Filename: "",
	//	Body: asset("apiacl.casbin"), CreateTime: now, UpdateTime: now, DeletedAt: "",
	//})
	//dao.AddEndpointID(process.Endpoint{
	//	ID: f(), Endpoint: "/_internal/adminacl", Methods: "ANY", MimeType: "", Filename: "",
	//	Body: asset("adminacl.casbin"), CreateTime: now, UpdateTime: now, DeletedAt: "",
	//})

	return nil
}

// SaveEndpoint ...
func SaveEndpoint(model process.APIDataModel) (*process.Endpoint, error) {
	if model.Endpoint == "" || model.Method == "" {
		return nil, fmt.Errorf("model endpoint and method could not be empty")
	}

	if err := TestAPIRouter(model); err != nil {
		return nil, err
	}

	defer SyncAPIRouter()

	var ep *process.Endpoint

	err := DBDo(func(dao *Dao) error {
		old := dao.FindEndpoint(model.ID)
		if old == nil {
			old = dao.FindByEndpoint(model.Endpoint)
		}

		bean := CreateEndpoint(model, old)

		if old == nil {
			lastInsertRowID := dao.AddEndpoint(bean)
			bean.ID = process.ID(fmt.Sprintf("%d", lastInsertRowID))
		} else {
			dao.UpdateEndpoint(bean)
		}

		ep = &bean

		return nil
	})

	return ep, err
}

// CreateAPIDataModel creates APIDataModel from Endpoint.
func CreateAPIDataModel(ep *process.Endpoint, query bool) *process.APIDataModel {
	if ep == nil {
		return nil
	}

	m := &process.APIDataModel{
		ID:       ep.ID,
		Endpoint: ep.Endpoint,
		Method:   ep.Methods,
		MimeType: ep.MimeType,
		Filename: ep.Filename,
	}

	if ep.Filename != "" {
		m.FileContent = []byte(ep.Body)
	} else {
		m.Body = ep.Body
	}

	if query {
		return m
	}

	m.TryDo(ep.CreateMockbin)
	m.TryDo(ep.CreateEcho)
	m.TryDo(ep.CreateProxy)
	m.TryDo(ep.CreateDirect)
	m.TryDo(ep.CreateDefault)

	return m
}

// CreateEndpoint creates an endpoint from APIDataModel.
func CreateEndpoint(model process.APIDataModel, old *process.Endpoint) process.Endpoint {
	now := util.TimeFmt(time.Now())
	body := model.Body

	if body == "" {
		body = string(model.FileContent)
	}

	ep := process.Endpoint{
		ID:         model.ID,
		Endpoint:   model.Endpoint,
		Methods:    model.Method,
		MimeType:   model.MimeType,
		Filename:   model.Filename,
		Body:       body,
		CreateTime: now,
		UpdateTime: now,
		DeletedAt:  "",
	}

	if old != nil {
		if old.Body != "" && ep.Body == "" {
			ep.Body = old.Body
		}

		if old.ID != "" && ep.ID == "" {
			ep.ID = old.ID
		}
	}

	return ep
}

// DeleteEndpoint ...
func DeleteEndpoint(id string) error {
	defer SyncAPIRouter()

	return DBDo(func(dao *Dao) error {
		dao.DeleteEndpoint(process.Endpoint{
			ID:        process.ID(id),
			DeletedAt: util.TimeFmt(time.Now()),
		})

		return nil
	})
}

// GetEndpoint ...
func GetEndpoint(id process.ID) (*process.APIDataModel, error) {
	var model *process.APIDataModel

	err := DBDo(func(dao *Dao) error {
		ep := dao.FindEndpoint(id)
		model = CreateAPIDataModel(ep, true)

		return nil
	})

	return model, err
}

// nolint gochecknoglobals
var (
	apiRouter     *gin.Engine
	apiRouterLock sync.Mutex

	broadcastThrottler = util.MakeThrottle(60, 60*time.Second)
)

func serveAPI(w http.ResponseWriter, r *http.Request) (v process.RouterResult) {
	apiRouterLock.Lock()
	router := apiRouter
	apiRouterLock.Unlock()

	ctx := context.WithValue(r.Context(), process.RouterResultKey, &v)
	hlog.StdLogWrapHandler(router).ServeHTTP(w, r.WithContext(ctx))

	return
}

// JoinContextPath joins the context path to elem.
func JoinContextPath(elem string) string {
	if Environments.ContextPath == "/" {
		return elem
	}

	return filepath.Join(Environments.ContextPath, elem)
}

// TestAPIRouter ...
func TestAPIRouter(p process.APIDataModel) error {
	router := sariaf.New()

	for _, ep := range EndpointList(false) {
		if ep.ID == p.ID {
			continue
		}

		if err := router.Handle(http.MethodGet, JoinContextPath(ep.Endpoint), nil); err != nil {
			return err
		}
	}

	return router.Handle(http.MethodGet, JoinContextPath(p.Endpoint), nil)
}

func echoXHeaders(c *gin.Context) {
	rh := c.Request.Header
	for k := range rh {
		if strings.HasPrefix(k, "X-") {
			c.Header(k, rh.Get(k))
		}
	}
}

// SyncAPIRouter ...
func SyncAPIRouter() {
	r := gin.New()
	r.Use(echoXHeaders)

	for _, ep := range EndpointList(false) {
		routing(r, ep)
	}

	r.NoRoute(noRouteHandlerWrap)

	apiRouterLock.Lock()
	apiRouter = r
	apiRouterLock.Unlock()
}

func routing(r *gin.Engine, ep process.APIDataModel) {
	if strings.HasPrefix(ep.Endpoint, "/_internal") {
		ep.InternalProcess(ep.Endpoint[10:])
		return
	}

	h := ep.HandleJSON
	if ep.MimeType != "" {
		h = ep.HandleFileDownload
	}

	if strings.EqualFold(ep.Method, "ANY") {
		r.Any(JoinContextPath(ep.Endpoint), h)
	} else {
		r.Handle(ep.Method, JoinContextPath(ep.Endpoint), h)
	}
}

func noRouteHandlerWrap(c *gin.Context) {
	cw := util.NewGinCopyWriter(c.Writer)
	c.Writer = cw

	processed := noRouteHandler(c)

	rr := c.Request.Context().Value(process.RouterResultKey).(*process.RouterResult)
	rr.RouterServed = processed
	rr.RouterBody = cw.Bytes()
	rr.RemoteAddr = c.Request.RemoteAddr
	rr.ResponseSize = cw.Size()
	rr.ResponseStatus = cw.Status()
	rr.ResponseHeader = util.ConvertHeader(cw.Header())
}

func noRouteHandler(c *gin.Context) (processed bool) {
	processed = true
	p := c.Request.URL.Path

	ua := user_agent.New(c.Request.UserAgent())
	isBrowser := ua.OS() != ""
	useJSON := util.HasContentType(c.Request, "application/json") || !isBrowser
	hl := strings.ToLower(c.Query("_hl"))
	if strings.HasSuffix(hl, ".json") {
		useJSON = true
		hl = hl[:len(hl)-5]
	}

	if strings.HasSuffix(p, ".json") {
		useJSON = true
		p = p[:len(p)-5]
	}

	switch {
	case hl == "v" || p == "/v":
		c.PureJSON(http.StatusOK, gin.H{"version": Version, "updateTime": UpdateTime})
	case hl == "curl" || p == "/curl":
		values := c.Request.URL.Query()
		delete(values, "_hl")
		c.Request.URL.RawQuery = values.Encode()
		cmd, _ := http2curl.GetCurlCmd(c.Request)
		c.Data(http.StatusOK, util.ContentTypeText, []byte(cmd.String()))
	case hl == "ip" || p == "/ip":
		process.ProcessIP(c, useJSON)
	case hl == "time" || p == "/time":
		if useJSON {
			c.PureJSON(http.StatusOK, gin.H{"time": util.TimeFmt(time.Now())})
		} else {
			c.Data(http.StatusOK, util.ContentTypeText, []byte(util.TimeFmt(time.Now())))
		}
	case hl == "sysinfo" || p == "/sysinfo":
		showsMap := make(map[string]bool)
		for _, p := range strings.Split("host,mem,cpu,disk,interf,ps", ",") {
			showsMap[p] = true
		}
		if useJSON {
			c.PureJSON(http.StatusOK, sysinfo.GetSysInfo(showsMap))
		} else {
			c.Status(http.StatusOK)
			c.Header("Content-Type", util.ContentTypeText)
			sysinfo.PrintTable(showsMap, "~", c.Writer)
		}
	case (hl == "" && p == "/") || hl == "echo" || p == "/echo":
		if useJSON {
			c.PureJSON(http.StatusOK, process.CreateRequestMap(c, nil))
		} else {
			d, _ := httputil.DumpRequest(c.Request, true)
			c.Data(http.StatusOK, util.ContentTypeText, d)
		}
	default:
		c.Status(http.StatusNotFound)
		processed = false
	}

	return
}

// EndpointList ...
func EndpointList(query bool) []process.APIDataModel {
	var endPoints []process.Endpoint

	_ = DBDo(func(dao *Dao) error {
		endPoints = dao.ListEndpoints()

		return nil
	})

	items := make([]process.APIDataModel, len(endPoints))

	for i, v := range endPoints {
		v := v
		items[i] = *CreateAPIDataModel(&v, query)
	}

	return items
}

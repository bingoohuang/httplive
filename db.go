package httplive

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bingoohuang/httplive/internal/process"
	"github.com/bingoohuang/httplive/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/markbates/pkger"

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

func asset(name string) string {
	pkger.Include("/assets")

	f, err := pkger.Open(filepath.Join("/assets", name))
	if err != nil {
		panic(err)
	}

	defer f.Close()

	buf := new(bytes.Buffer)
	_, _ = io.Copy(buf, f)

	return buf.String()
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
	dao.AddEndpointID(process.Endpoint{
		ID: "0", Endpoint: "/api/demo", Methods: http.MethodGet, MimeType: "", Filename: "",
		Body: asset("apidemo.json"), CreateTime: now, UpdateTime: now, DeletedAt: "",
	})
	dao.AddEndpointID(process.Endpoint{
		ID: "1", Endpoint: "/dynamic/demo", Methods: http.MethodPost, MimeType: "", Filename: "",
		Body: asset("dynamicdemo.json"), CreateTime: now, UpdateTime: now, DeletedAt: "",
	})
	dao.AddEndpointID(process.Endpoint{
		ID: "2", Endpoint: "/proxy/demo", Methods: http.MethodGet, MimeType: "", Filename: "",
		Body: asset("proxydemo.json"), CreateTime: now, UpdateTime: now, DeletedAt: "",
	})
	dao.AddEndpointID(process.Endpoint{
		ID: "3", Endpoint: "/echo/:id", Methods: "ANY", MimeType: "", Filename: "",
		Body: asset("echo.json"), CreateTime: now, UpdateTime: now, DeletedAt: "",
	})
	dao.AddEndpointID(process.Endpoint{
		ID: "4", Endpoint: "/mockbin", Methods: "ANY", MimeType: "", Filename: "",
		Body: asset("mockbin.json"), CreateTime: now, UpdateTime: now, DeletedAt: "",
	})

	return nil
}

// SaveEndpoint ...
func SaveEndpoint(model process.APIDataModel) (*process.Endpoint, error) {
	if model.Endpoint == "" || model.Method == "" {
		return nil, fmt.Errorf("model endpoint and method could not be empty")
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

	if m.ServeFn == nil {
		ep.CreateMockbin(m)
	}
	if m.ServeFn == nil {
		ep.CreateEcho(m)
	}
	if m.ServeFn == nil {
		ep.CreateProxy(m)
	}
	if m.ServeFn == nil {
		ep.CreateDirect(m)
	}
	if m.ServeFn == nil {
		ep.CreateDefault(m)
	}

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
	router.ServeHTTP(w, r.WithContext(ctx))

	return
}

// JoinContextPath joins the context path to elem.
func JoinContextPath(elem string) string {
	if Environments.ContextPath == "/" {
		return elem
	}

	return filepath.Join(Environments.ContextPath, elem)
}

// SyncAPIRouter ...
func SyncAPIRouter() {
	router := gin.New()

	for _, ep := range EndpointList(false) {
		h := ep.HandleJSON
		if ep.MimeType != "" {
			h = ep.HandleFileDownload
		}

		if strings.EqualFold(ep.Method, "ANY") {
			router.Any(JoinContextPath(ep.Endpoint), h)
		} else {
			router.Handle(ep.Method, JoinContextPath(ep.Endpoint), h)
		}
	}

	apiRouterLock.Lock()
	apiRouter = router
	apiRouterLock.Unlock()
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

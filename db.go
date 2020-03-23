package httplive

import (
	"bytes"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/bingoohuang/sqlx"
	"github.com/julienschmidt/httprouter"
	_ "github.com/mattn/go-sqlite3" // import sqlite3
)

const sqlstr = `
-- name: CreateTable
CREATE TABLE IF NOT EXISTS httplive_endpoint (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	endpoint TEXT NOT NULL UNIQUE,
	methods TEXT NOT NULL,
	mime_type TEXT,
	filename TEXT,
	body TEXT,
	create_time TEXT NOT NULL,
	update_time TEXT NOT NULL,
	deleted_at TEXT NULL
);

-- name: FindEndpoint
SELECT id, endpoint, methods, mime_type, filename, body, create_time, update_time, deleted_at
FROM httplive_endpoint
WHERE id = :1;

-- name: ListEndpoints
SELECT id, endpoint, methods, mime_type, filename, body, create_time, update_time, deleted_at
FROM httplive_endpoint
WHERE deleted_at = '' or deleted_at IS NULL 
ORDER BY id;

-- name: AddEndpoint
INSERT INTO httplive_endpoint(endpoint, methods, mime_type, filename, body, create_time, update_time, deleted_at)
VALUES (:endpoint, :methods, :mime_type, :filename, :body, :create_time, :update_time, :deleted_at);

-- name: AddEndpointID
INSERT INTO httplive_endpoint(id, endpoint, methods, mime_type, filename, body, create_time, update_time, deleted_at)
VALUES (:id, :endpoint, :methods, :mime_type, :filename, :body, :create_time, :update_time, :deleted_at);

-- name: UpdateEndpoint
UPDATE httplive_endpoint
SET endpoint = :endpoint, methods = :methods, mime_type = :mime_type, filename = :filename,
	body = :body, create_time = :create_time, update_time = :update_time, deleted_at = :deleted_at
WHERE id = :id;

-- name: DeleteEndpoint
UPDATE httplive_endpoint
SET deleted_at = :deleted_at
WHERE id = :id;
`

// Endpoint is the structure for table httplive_endpoint.
type Endpoint struct {
	ID         ID     `name:"id"`
	Endpoint   string `name:"endpoint"`
	Methods    string `name:"methods"`
	MimeType   string `name:"mime_type"`
	Filename   string `name:"filename"`
	Body       string `name:"body"`
	CreateTime string `name:"create_time"`
	UpdateTime string `name:"update_time"`
	DeletedAt  string `name:"deleted_at"`
}

// Dao defines the api to access the database.
type Dao struct {
	CreateTable    func()
	ListEndpoints  func() []Endpoint
	FindEndpoint   func(ID ID) *Endpoint
	AddEndpoint    func(Endpoint)
	AddEndpointID  func(Endpoint)
	UpdateEndpoint func(Endpoint)
	DeleteEndpoint func(Endpoint)
	Logger         *sqlx.DaoLogrus
}

// CreateDao creates a dao.
func CreateDao(db *sql.DB) (*Dao, error) {
	dao := &Dao{Logger: &sqlx.DaoLogrus{}}
	err := sqlx.CreateDao(db, dao, sqlx.WithSQLStr(sqlstr))

	return dao, err
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
func CreateDB() error {
	if err := DBDo(createDB); err != nil {
		return err
	}

	SyncEndpointRouter()

	return nil
}

func createDB(dao *Dao) error {
	dao.CreateTable()

	demo := dao.FindEndpoint("0")
	if demo == nil {
		const body = `{
	"array": [
		1,
		2,
		3
	],
	"boolean": true,
	"null": null,
	"number": 123,
	"object": {
		"a": "b",
		"c": "d",
		"e": "f"
	},
	"string": "Hello World"
}`

		now := time.Now().Format("2006-01-02 15:04:05.000")
		ep := Endpoint{
			ID:         "0",
			Endpoint:   "/api/demo",
			Methods:    http.MethodGet,
			MimeType:   "",
			Filename:   "",
			Body:       body,
			CreateTime: now,
			UpdateTime: now,
			DeletedAt:  "",
		}

		dao.AddEndpointID(ep)
	}

	return nil
}

// SaveEndpoint ...
func SaveEndpoint(model APIDataModel) error {
	if model.Endpoint == "" || model.Method == "" {
		return fmt.Errorf("model endpoint and method could not be empty")
	}

	defer SyncEndpointRouter()

	return DBDo(func(dao *Dao) error {
		old := dao.FindEndpoint(model.ID)
		bean := CreateEndpoint(model, old)
		if old == nil {
			dao.AddEndpoint(bean)
		} else {
			dao.UpdateEndpoint(bean)
		}

		return nil
	})
}

// CreateAPIDataModel creates APIDataModel from Endpoint
func CreateAPIDataModel(ep *Endpoint) *APIDataModel {
	if ep == nil {
		return nil
	}

	m := &APIDataModel{
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

	return m
}

// CreateEndpoint creates an endpoint from APIDataModel
func CreateEndpoint(model APIDataModel, old *Endpoint) Endpoint {
	now := time.Now().Format("2006-01-02 15:04:05.000")
	body := model.Body

	if body == "" {
		body = string(model.FileContent)
	}

	ep := Endpoint{
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
	}

	return ep
}

// DeleteEndpoint ...
func DeleteEndpoint(id string) error {
	defer SyncEndpointRouter()

	return DBDo(func(dao *Dao) error {
		dao.DeleteEndpoint(Endpoint{ID: ID(id), DeletedAt: time.Now().Format("2006-01-02 15:04:05.000")})

		return nil
	})
}

// GetEndpoint ...
func GetEndpoint(id ID) (*APIDataModel, error) {
	var model *APIDataModel

	err := DBDo(func(dao *Dao) error {
		ep := dao.FindEndpoint(id)
		model = CreateAPIDataModel(ep)

		return nil
	})

	return model, err
}

// nolint gochecknoglobals
var (
	endpointRouter       *httprouter.Router
	endpointRouterLock   sync.Mutex
	endpointRouterServed bool

	boradcastThrottler = MakeThrottle(1 * time.Second)
)

// Throttle ...
type Throttle struct {
	tokenC chan bool
	stopC  chan bool
}

// MakeThrottle ...
func MakeThrottle(duration time.Duration) *Throttle {
	t := &Throttle{
		tokenC: make(chan bool, 1),
		stopC:  make(chan bool, 1),
	}

	go func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()

		for {
			select {
			case <-t.stopC:
				return
			case <-ticker.C:
				select {
				case t.tokenC <- true:
				default:
				}
			}
		}
	}()

	return t
}

// Stop ...
func (t *Throttle) Stop() {
	t.stopC <- true
}

// Allow ...
func (t *Throttle) Allow() bool {
	select {
	case <-t.tokenC:
		return true
	default:
		return false
	}
}

// EndpointServeHTTP ...
func EndpointServeHTTP(w http.ResponseWriter, r *http.Request) bool {
	endpointRouterLock.Lock()
	defer endpointRouterLock.Unlock()

	endpointRouterServed = false

	endpointRouter.ServeHTTP(w, r)

	return endpointRouterServed
}

// SyncEndpointRouter ...
func SyncEndpointRouter() {
	endpointRouterLock.Lock()
	defer endpointRouterLock.Unlock()

	endpointRouter = httprouter.New()

	for _, endpoint := range EndpointList() {
		ep := endpoint
		if ep.MimeType == "" {
			b := []byte(ep.Body)
			endpointRouter.Handle(ep.Method, ep.Endpoint,
				func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
					endpointRouterServed = true
					w.WriteHeader(http.StatusOK)
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					_, _ = w.Write(b)
				})
		} else {
			endpointRouter.Handle(ep.Method, ep.Endpoint,
				func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
					endpointRouterServed = true
					w.WriteHeader(http.StatusOK)
					w.Header().Set("Content-Disposition", `attachment; filename="`+ep.Filename+`"`)
					reader := bytes.NewReader(ep.FileContent)
					http.ServeContent(w, r, ep.Filename, time.Now(), reader)
				})
		}
	}
}

// EndpointList ...
func EndpointList() []APIDataModel {
	var endPoints []Endpoint

	_ = DBDo(func(dao *Dao) error {
		endPoints = dao.ListEndpoints()

		return nil
	})

	items := make([]APIDataModel, len(endPoints))

	for i, v := range endPoints {
		v := v
		items[i] = *CreateAPIDataModel(&v)
	}

	return items
}

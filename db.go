package httplive

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Knetic/govaluate"
	"github.com/tidwall/gjson"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bingoohuang/sqlx"
	"github.com/gobuffalo/packr/v2"
	"github.com/julienschmidt/httprouter"
	_ "github.com/mattn/go-sqlite3" // import sqlite3
)

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
	CreateTable     func()
	ListEndpoints   func() []Endpoint
	FindEndpoint    func(ID ID) *Endpoint
	FindByEndpoint  func(endpoint string) *Endpoint
	AddEndpoint     func(Endpoint) int
	LastInsertRowID func() ID
	AddEndpointID   func(Endpoint)
	UpdateEndpoint  func(Endpoint)
	DeleteEndpoint  func(Endpoint)
	Logger          sqlx.DaoLogger
}

// CreateDao creates a dao.
func CreateDao(db *sql.DB) (*Dao, error) {
	dao := &Dao{Logger: &sqlx.DaoLogrus{}}

	box := packr.New("myBox", "assets")

	s, err := box.FindString("httplive.sql")
	if err != nil {
		return nil, err
	}

	err = sqlx.CreateDao(dao, sqlx.WithDB(db), sqlx.WithSQLStr(s))

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
	if demo != nil {
		return nil
	}

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
	dao.AddEndpointID(Endpoint{
		ID:         "0",
		Endpoint:   "/api/demo",
		Methods:    http.MethodGet,
		MimeType:   "",
		Filename:   "",
		Body:       body,
		CreateTime: now,
		UpdateTime: now,
		DeletedAt:  "",
	})
	dao.AddEndpointID(Endpoint{
		ID:       "1",
		Endpoint: "/dynamic/demo",
		Methods:  http.MethodPost,
		MimeType: "",
		Filename: "",
		Body: `
{
  "dynamic": [
    {
      "condition":"json_name == 'bingoo'",
      "response": {
        "name":"bingoo"
      }
    },
    {
      "condition":"json_name == 'huang'",
      "response": {
        "name":"huang",
        "age":100
      }
    },
    {
      "condition":"json_name == 'ding' && json_age == 10",
      "response": {
        "name":"xxx",
        "age":100,
        "xxx":3000
      }
    }
    ,
    {
      "condition":"json_name == 'ding' && json_age == 20",
      "response": {
        "name":"xxx",
        "age":100,
        "xxx":3000
      }
    }
  ]
}
`,
		CreateTime: now,
		UpdateTime: now,
		DeletedAt:  "",
	})

	return nil
}

// SaveEndpoint ...
func SaveEndpoint(model APIDataModel) (*Endpoint, error) {
	if model.Endpoint == "" || model.Method == "" {
		return nil, fmt.Errorf("model endpoint and method could not be empty")
	}

	defer SyncEndpointRouter()

	var ep *Endpoint

	err := DBDo(func(dao *Dao) error {
		old := dao.FindEndpoint(model.ID)
		if old == nil {
			old = dao.FindByEndpoint(model.Endpoint)
		}

		bean := CreateEndpoint(model, old)

		if old == nil {
			lastInsertRowID := dao.AddEndpoint(bean)
			bean.ID = ID(fmt.Sprintf("%d", lastInsertRowID))
		} else {
			dao.UpdateEndpoint(bean)
		}

		ep = &bean

		return nil
	})

	return ep, err
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

		if old.ID != "" && ep.ID == "" {
			ep.ID = old.ID
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
	endpointRouterBody   []byte

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

func CompactJSON(data []byte) []byte {
	compactedBuffer := new(bytes.Buffer)
	if err := json.Compact(compactedBuffer, data); err != nil {
		return data
	}

	return compactedBuffer.Bytes()
}

// EndpointServeHTTP ...
func EndpointServeHTTP(w http.ResponseWriter, r *http.Request) (bool, []byte) {
	endpointRouterLock.Lock()
	defer endpointRouterLock.Unlock()

	endpointRouterServed = false
	endpointRouterBody = nil

	endpointRouter.ServeHTTP(w, r)

	return endpointRouterServed, endpointRouterBody
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
				func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
					endpointRouterServed = true
					w.WriteHeader(http.StatusOK)
					w.Header().Set("Content-Type", "application/json; charset=utf-8")

					if !dynamic(ep.Body, r, w, p) {
						endpointRouterBody = b
						_, _ = w.Write(b)
					}

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

type DynamicValue struct {
	Condition string          `json:"condition"`
	Response  json.RawMessage `json:"response"`
}

func dynamic(respBody string, r *http.Request, w http.ResponseWriter, p httprouter.Params) bool {
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println(err)
		//log.Printf("Error reading body: %v", err)
		//http.Error(w, "can't read body", http.StatusBadRequest)
		return false
	}

	// Work / inspect body. You may even modify it!

	// And now set a new body, which will simulate the same data we read:
	r.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody))

	if !strings.HasPrefix(r.URL.Path, "/dynamic") {
		return false
	}

	dynamic := gjson.Get(respBody, "dynamic")
	if dynamic.Type != gjson.JSON {
		return false
	}

	var dynamicValues []DynamicValue
	if err := json.Unmarshal([]byte(dynamic.Raw), &dynamicValues); err != nil {
		fmt.Println(err)
		return false
	}

	for _, v := range dynamicValues {
		expr, err := govaluate.NewEvaluableExpression(v.Condition)
		if err != nil {
			fmt.Println(err)
			return false
		}

		parameters := make(map[string]interface{})
		for _, va := range expr.Vars() {
			if strings.HasPrefix(va, "json_") {
				parameters[va] = gjson.GetBytes(reqBody, va[5:]).Value()
			} else if strings.HasPrefix(va, "query_") {
				parameters[va] = r.URL.Query().Get(va[6:])
			} else if strings.HasPrefix(va, "router_") {
				// /user/:user
				parameters[va] = p.ByName(va[7:])
			} else if strings.HasPrefix(va, "header_") {
				parameters[va] = r.Header.Get(va[7:])
			} else {
				va = gjson.Get(respBody, va).String()
				var vaValue interface{} = nil
				if strings.HasPrefix(va, "json:") {
					vaValue = gjson.GetBytes(reqBody, va[5:]).Value()
				} else if strings.HasPrefix(va, "query:") {
					vaValue = r.URL.Query().Get(va[6:])
				} else if strings.HasPrefix(va, "router:") {
					// /user/:user
					vaValue = p.ByName(va[7:])
				} else if strings.HasPrefix(va, "header:") {
					vaValue = r.Header.Get(va[7:])
				}

				parameters[va] = vaValue
			}
		}

		evaluateResult, err := expr.Evaluate(parameters)
		if err != nil {
			fmt.Println(err)
			return false
		}

		if yes, ok := evaluateResult.(bool); ok && yes {
			endpointRouterBody = v.Response
			_, _ = w.Write(v.Response)
			return true
		}
	}

	return false
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

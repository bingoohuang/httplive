package httplive

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Knetic/govaluate"
	"github.com/tidwall/gjson"

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
		Body: `{
  "name": "json:name",
  "age": "json:age",
  "dynamic": [
    {
      "condition":"name == 'bingoo'",
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
      "condition":"name == 'ding' && age == 10",
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
      },
      "status": 202,
      "headers": {
        "xxx": "yyy",
        "Content-Type": "text/plain; charset=utf-8"
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

	dynamic := gjson.Get(ep.Body, "dynamic")
	isDynamic := dynamic.Type == gjson.JSON && strings.HasPrefix(dynamic.Raw, "[")
	if isDynamic {
		m.dynamicValuers = createDynamics(ep.Body, []byte(dynamic.Raw))
	}

	if ep.Filename != "" {
		m.FileContent = []byte(ep.Body)
	} else {
		m.Body = ep.Body
	}

	return m
}

func createDynamics(epBody string, dynamicRaw []byte) (dynamicValues []DynamicValue) {
	if err := json.Unmarshal(dynamicRaw, &dynamicValues); err != nil {
		fmt.Println(err)
		return
	}

	for i, v := range dynamicValues {
		expr, err := govaluate.NewEvaluableExpression(v.Condition)
		if err != nil {
			fmt.Println(err)
			return
		}

		dynamicValues[i].expr = expr
		dynamicValues[i].parametersEvaluator = makeParameters(epBody, expr)
	}

	return
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
	endpointRouter     *httprouter.Router
	endpointRouterLock sync.Mutex

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

type RouterResult struct {
	RouterServed bool
	RouterBody   []byte
	Filename     string
}

type ContextKey int

const routerResultKey ContextKey = iota

// EndpointServeHTTP ...
func EndpointServeHTTP(w http.ResponseWriter, r *http.Request) RouterResult {
	endpointRouterLock.Lock()
	router := endpointRouter
	endpointRouterLock.Unlock()

	v := RouterResult{}

	ctx := context.WithValue(r.Context(), routerResultKey, &v)
	router.ServeHTTP(w, r.WithContext(ctx))

	return v
}

func JoinContextPath(s string) string {
	if Environments.ContextPath == "/" {
		return s
	}

	return filepath.Join(Environments.ContextPath, s)
}

// SyncEndpointRouter ...
func SyncEndpointRouter() {
	router := httprouter.New()

	for _, endpoint := range EndpointList() {
		ep := endpoint
		path := JoinContextPath(ep.Endpoint)
		if ep.MimeType == "" {
			router.Handle(ep.Method, path, ep.HandleJSON)
		} else {
			router.Handle(ep.Method, path, ep.HandleFileDownload)
		}
	}

	endpointRouterLock.Lock()
	endpointRouter = router
	endpointRouterLock.Unlock()
}

func (ep APIDataModel) HandleFileDownload(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	routerResult := r.Context().Value(routerResultKey).(*RouterResult)
	routerResult.RouterServed = true
	routerResult.Filename = ep.Filename

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Disposition", `attachment; filename="`+ep.Filename+`"`)
	reader := bytes.NewReader(ep.FileContent)
	http.ServeContent(w, r, ep.Filename, time.Now(), reader)
}

func (ep APIDataModel) HandleJSON(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	routerResult := r.Context().Value(routerResultKey).(*RouterResult)
	routerResult.RouterServed = true

	if !dynamic(ep, r, w, p, routerResult) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		b := []byte(ep.Body)
		routerResult.RouterBody = b
		_, _ = w.Write(b)
	}
}

type DynamicValue struct {
	Condition string            `json:"condition"`
	Response  json.RawMessage   `json:"response"`
	Status    int               `json:"status"`
	Headers   map[string]string `json:"headers"`

	expr                *govaluate.EvaluableExpression `json:"-"`
	parametersEvaluator map[string]Valuer              `json:"-"`
}

func dynamic(ep APIDataModel, r *http.Request, w http.ResponseWriter, p httprouter.Params, routerResult *RouterResult) bool {
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

	if len(ep.dynamicValuers) == 0 {
		return false
	}

	for _, v := range ep.dynamicValuers {
		parameters := make(map[string]interface{}, len(v.parametersEvaluator))
		for k, valuer := range v.parametersEvaluator {
			parameters[k] = valuer(r, reqBody, p)
		}

		evaluateResult, err := v.expr.Evaluate(parameters)
		if err != nil {
			fmt.Println(err)
			return false
		}

		if yes, ok := evaluateResult.(bool); ok && yes {
			v.respsoneDynamic(w, routerResult)

			return true
		}
	}

	return false
}

func (v DynamicValue) respsoneDynamic(w http.ResponseWriter, routerResult *RouterResult) {
	if v.Status != 0 {
		w.WriteHeader(v.Status)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	contentTypeSet := false
	header := w.Header()
	for k, v := range v.Headers {
		if strings.EqualFold(k, "Content-Type") {
			contentTypeSet = true
		}
		header.Set(k, v)
	}

	if !contentTypeSet {
		if bytes.HasPrefix(v.Response, []byte("{")) || bytes.HasPrefix(v.Response, []byte("[")) {
			header.Set("Content-Type", "application/json; charset=utf-8")
		} else {
			header.Set("Content-Type", "text/plain; charset=utf-8")
		}
	}

	routerResult.RouterBody = v.Response
	_, _ = w.Write(v.Response)
}

func makeParameters(respBody string, expr *govaluate.EvaluableExpression) map[string]Valuer {
	parameters := make(map[string]Valuer)
	for _, va := range expr.Vars() {
		if strings.HasPrefix(va, "json_") {
			k := va[5:]

			parameters[va] = func(r *http.Request, reqBody []byte, p httprouter.Params) interface{} {
				return gjson.GetBytes(reqBody, k).Value()
			}

		} else if strings.HasPrefix(va, "query_") {
			k := va[6:]
			parameters[va] = func(r *http.Request, reqBody []byte, p httprouter.Params) interface{} {
				return r.URL.Query().Get(k)
			}
		} else if strings.HasPrefix(va, "router_") {
			// /user/:user
			k := va[7:]
			parameters[va] = func(r *http.Request, reqBody []byte, p httprouter.Params) interface{} {
				return p.ByName(k)
			}
		} else if strings.HasPrefix(va, "header_") {
			k := va[7:]
			parameters[va] = func(r *http.Request, reqBody []byte, p httprouter.Params) interface{} {
				return r.Header.Get(k)
			}
		} else {
			indirectVa := gjson.Get(respBody, va).String()

			parameters[va] = func(r *http.Request, reqBody []byte, p httprouter.Params) interface{} {
				if strings.HasPrefix(indirectVa, "json:") {
					return gjson.GetBytes(reqBody, indirectVa[5:]).Value()
				} else if strings.HasPrefix(indirectVa, "query:") {
					return r.URL.Query().Get(indirectVa[6:])
				} else if strings.HasPrefix(indirectVa, "router:") {
					// /user/:user
					return p.ByName(indirectVa[7:])
				} else if strings.HasPrefix(indirectVa, "header:") {
					return r.Header.Get(indirectVa[7:])
				}

				return nil
			}
		}
	}

	return parameters
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

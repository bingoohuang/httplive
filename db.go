package httplive

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
	"github.com/markbates/pkger"

	"github.com/Knetic/govaluate"
	"github.com/tidwall/gjson"

	"github.com/bingoohuang/sqlx"
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
	err := sqlx.CreateDao(dao, sqlx.WithDB(db), sqlx.WithSQLStr(boxString("httplive.sql")))

	return dao, err
}

func boxString(name string) string {
	pkger.Include("/assets") // nolint:staticcheck

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

	now := time.Now().Format("2006-01-02 15:04:05.000")
	dao.AddEndpointID(Endpoint{
		ID:         "0",
		Endpoint:   "/api/demo",
		Methods:    http.MethodGet,
		MimeType:   "",
		Filename:   "",
		Body:       boxString("apidemo.json"),
		CreateTime: now,
		UpdateTime: now,
		DeletedAt:  "",
	})
	dao.AddEndpointID(Endpoint{
		ID:         "1",
		Endpoint:   "/dynamic/demo",
		Methods:    http.MethodPost,
		MimeType:   "",
		Filename:   "",
		Body:       boxString("dynamicdemo.json"),
		CreateTime: now,
		UpdateTime: now,
		DeletedAt:  "",
	})

	dao.AddEndpointID(Endpoint{
		ID:         "2",
		Endpoint:   "/proxy/demo",
		Methods:    http.MethodGet,
		MimeType:   "",
		Filename:   "",
		Body:       boxString("proxydemo.json"),
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

	defer SyncAPIRouter()

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

// CreateAPIDataModel creates APIDataModel from Endpoint.
func CreateAPIDataModel(ep *Endpoint, query bool) *APIDataModel {
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

	if query {
		return m
	}

	ep.createDirect(m)
	ep.createDynamicValuers(m)
	ep.createProxy(m)

	return m
}

func (ep *Endpoint) createDirect(m *APIDataModel) {
	direct := gjson.Get(ep.Body, "_direct")
	isDirect := direct.Type != gjson.Null
	if !isDirect {
		return
	}

	m.serveFn = func(c *gin.Context) {
		rsp := []byte(direct.String())
		c.Data(http.StatusOK, detectContentType(rsp), rsp)
	}
}

func (ep *Endpoint) createDynamicValuers(m *APIDataModel) {
	dynamic := gjson.Get(ep.Body, "_dynamic")
	isDynamic := dynamic.Type == gjson.JSON && strings.HasPrefix(dynamic.Raw, "[")
	if !isDynamic {
		return
	}

	m.dynamicValuers = createDynamics(ep.Body, []byte(dynamic.Raw))
}

func (ep *Endpoint) createProxy(m *APIDataModel) {
	proxy := gjson.Get(ep.Body, "_proxy")
	isProxy := proxy.Type == gjson.String && strings.HasPrefix(proxy.String(), "http")
	if !isProxy {
		return
	}

	p, err := url.Parse(proxy.String())
	if err != nil {
		fmt.Println(err)
		return
	}

	m.serveFn = func(c *gin.Context) {
		originalPath := c.Request.URL.String()
		rp := ReverseProxy(originalPath, p.Host, p.Path, 30*time.Second)
		rp.ServeHTTP(c.Writer, c.Request)
	}
}

func createDynamics(epBody string, dynamicRaw []byte) (dynamicValues []dynamicValue) {
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

// CreateEndpoint creates an endpoint from APIDataModel.
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
	defer SyncAPIRouter()

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
		model = CreateAPIDataModel(ep, true)

		return nil
	})

	return model, err
}

// nolint gochecknoglobals
var (
	apiRouter     *gin.Engine
	apiRouterLock sync.Mutex

	broadcastThrottler = MakeThrottle(1 * time.Second)
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

func compactJSON(data []byte) []byte {
	compactedBuffer := new(bytes.Buffer)
	if err := json.Compact(compactedBuffer, data); err != nil {
		logrus.Warnf("json.Compact error: %v", err)

		v, _ := json.Marshal(map[string]string{
			"raw": string(data),
		})
		return v
	}

	return compactedBuffer.Bytes()
}

type routerResult struct {
	RouterServed bool
	RouterBody   []byte
	Filename     string
}

type contextKey int

const routerResultKey contextKey = iota

func serveAPI(w http.ResponseWriter, r *http.Request) routerResult {
	apiRouterLock.Lock()
	router := apiRouter
	apiRouterLock.Unlock()

	v := routerResult{}

	ctx := context.WithValue(r.Context(), routerResultKey, &v)
	router.ServeHTTP(w, r.WithContext(ctx))

	return v
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

	for _, endpoint := range EndpointList(false) {
		ep := endpoint
		path := JoinContextPath(ep.Endpoint)

		if strings.EqualFold(ep.Method, "ANY") {
			if ep.MimeType == "" {
				router.Any(path, ep.handleJSON)
			} else {
				router.Any(path, ep.handleFileDownload)
			}
		} else {
			if ep.MimeType == "" {
				router.Handle(ep.Method, path, ep.handleJSON)
			} else {
				router.Handle(ep.Method, path, ep.handleFileDownload)
			}
		}
	}

	apiRouterLock.Lock()
	apiRouter = router
	apiRouterLock.Unlock()
}

func (ep APIDataModel) handleFileDownload(c *gin.Context) {
	routerResult := c.Request.Context().Value(routerResultKey).(*routerResult)
	routerResult.RouterServed = true
	routerResult.Filename = ep.Filename
	c.Status(http.StatusOK)

	if c.Query("_view") == "" {
		h := c.Header
		h("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": ep.Filename}))
		h("Content-Description", "File Transfer")
		h("Content-Type", "application/octet-stream")
		h("Content-Transfer-Encoding", "binary")
		h("Expires", "0")
		h("Cache-Control", "must-revalidate")
		h("Pragma", "public")
	}

	http.ServeContent(c.Writer, c.Request, ep.Filename, time.Now(), bytes.NewReader(ep.FileContent))
}

func (ep APIDataModel) handleJSON(c *gin.Context) {
	routerResult := c.Request.Context().Value(routerResultKey).(*routerResult)
	routerResult.RouterServed = true

	if ep.serveFn != nil {
		ep.serveFn(c)
		return
	}

	if !routerResult.dynamic(ep, c) {
		b := []byte(ep.Body)
		routerResult.RouterBody = b
		c.Data(http.StatusOK, detectContentType(b), b)
	}
}

type dynamicValue struct {
	Condition string            `json:"condition"`
	Response  json.RawMessage   `json:"response"`
	Status    int               `json:"status"`
	Headers   map[string]string `json:"headers"`

	expr                *govaluate.EvaluableExpression
	parametersEvaluator map[string]valuer
}

func (rr *routerResult) dynamic(ep APIDataModel, c *gin.Context) bool {
	if len(ep.dynamicValuers) == 0 {
		return false
	}

	reqBody, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		fmt.Println(err)
		return false
	}

	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody))

	for _, v := range ep.dynamicValuers {
		parameters := make(map[string]interface{}, len(v.parametersEvaluator))
		for k, valuer := range v.parametersEvaluator {
			parameters[k] = valuer(reqBody, c)
		}

		evaluateResult, err := v.expr.Evaluate(parameters)
		if err != nil {
			fmt.Println(err)
			return false
		}

		if yes, ok := evaluateResult.(bool); ok && yes {
			v.responseDynamic(c, rr)

			return true
		}
	}

	return false
}

func (v dynamicValue) responseDynamic(c *gin.Context, routerResult *routerResult) {
	statusCode := v.Status
	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	contentType := ""
	for k, v := range v.Headers {
		if strings.EqualFold(k, "Content-Type") {
			contentType = v
		} else {
			c.Header(k, v)
		}
	}

	if contentType == "" {
		contentType = detectContentType(v.Response)
	}

	routerResult.RouterBody = v.Response
	c.Data(statusCode, contentType, v.Response)
}

func detectContentType(rsp []byte) string {
	if bytes.HasPrefix(rsp, []byte("{")) || bytes.HasPrefix(rsp, []byte("[")) {
		return "application/json; charset=utf-8"
	}

	return "text/plain; charset=utf-8"
}

func makeParameters(respBody string, expr *govaluate.EvaluableExpression) map[string]valuer {
	parameters := make(map[string]valuer)
	for _, va := range expr.Vars() {
		if strings.HasPrefix(va, "json_") {
			k := va[5:]

			parameters[va] = func(reqBody []byte, c *gin.Context) interface{} {
				return gjson.GetBytes(reqBody, k).Value()
			}
		} else if strings.HasPrefix(va, "query_") {
			k := va[6:]
			parameters[va] = func(reqBody []byte, c *gin.Context) interface{} {
				return c.Query(k)
			}
		} else if strings.HasPrefix(va, "router_") {
			// /user/:user
			k := va[7:]
			parameters[va] = func(reqBody []byte, c *gin.Context) interface{} {
				return c.Param(k)
			}
		} else if strings.HasPrefix(va, "header_") {
			k := va[7:]
			parameters[va] = func(reqBody []byte, c *gin.Context) interface{} {
				return c.GetHeader(k)
			}
		} else {
			indirectVa := gjson.Get(respBody, va).String()

			parameters[va] = func(reqBody []byte, c *gin.Context) interface{} {
				if strings.HasPrefix(indirectVa, "json:") {
					return gjson.GetBytes(reqBody, indirectVa[5:]).Value()
				} else if strings.HasPrefix(indirectVa, "query:") {
					return c.Query(indirectVa[6:])
				} else if strings.HasPrefix(indirectVa, "router:") {
					// /user/:user
					return c.Param(indirectVa[7:])
				} else if strings.HasPrefix(indirectVa, "header:") {
					return c.GetHeader(indirectVa[7:])
				}

				return nil
			}
		}
	}

	return parameters
}

// EndpointList ...
func EndpointList(query bool) []APIDataModel {
	var endPoints []Endpoint

	_ = DBDo(func(dao *Dao) error {
		endPoints = dao.ListEndpoints()

		return nil
	})

	items := make([]APIDataModel, len(endPoints))

	for i, v := range endPoints {
		v := v
		items[i] = *CreateAPIDataModel(&v, query)
	}

	return items
}

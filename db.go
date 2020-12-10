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
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	err := sqlx.CreateDao(dao, sqlx.WithDB(db), sqlx.WithSQLStr(boxStr("httplive.sql")))

	return dao, err
}

func boxStr(name string) string {
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

	now := time.Now().Format("2006-01-02 15:04:05.000")
	dao.AddEndpointID(Endpoint{
		ID: "0", Endpoint: "/api/demo", Methods: http.MethodGet, MimeType: "", Filename: "",
		Body: boxStr("apidemo.json"), CreateTime: now, UpdateTime: now, DeletedAt: "",
	})
	dao.AddEndpointID(Endpoint{
		ID: "1", Endpoint: "/dynamic/demo", Methods: http.MethodPost, MimeType: "", Filename: "",
		Body: boxStr("dynamicdemo.json"), CreateTime: now, UpdateTime: now, DeletedAt: "",
	})
	dao.AddEndpointID(Endpoint{
		ID: "2", Endpoint: "/proxy/demo", Methods: http.MethodGet, MimeType: "", Filename: "",
		Body: boxStr("proxydemo.json"), CreateTime: now, UpdateTime: now, DeletedAt: "",
	})
	dao.AddEndpointID(Endpoint{
		ID: "3", Endpoint: "/echo/:id", Methods: "ANY", MimeType: "", Filename: "",
		Body: `{"_echo": "JSON"}`, CreateTime: now, UpdateTime: now, DeletedAt: "",
	})

	dao.AddEndpointID(Endpoint{
		ID: "4", Endpoint: "/mockbin", Methods: "ANY", MimeType: "", Filename: "",
		Body: `{"status":200,"method":"GET",
	"headers":{"name":"bingoo"},"cookies":[{"name":"bingoo","value":"huang","maxAge":0, "path":"/","domain":"127.0.0.1","secure":false,"httpOnly":false}], "close": true,
	"contentType":"application/json; charset=utf-8", "payload":{"name":"bingoo"}} `, CreateTime: now, UpdateTime: now, DeletedAt: "",
	})
	return nil
}

// MockbinCookie defines the cookie format.
type MockbinCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	MaxAge   int    `json:"maxAge"`
	Path     string `json:"path"`
	Domain   string `json:"domain"`
	Secure   bool   `json:"secure"`
	HTTPOnly bool   `json:"httpOnly"`
}

// Mockbin defines the mockbin struct.
type Mockbin struct {
	Status      int               `json:"status"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
	Cookies     []MockbinCookie   `json:"cookies"`
	Close       bool              `json:"close"`
	ContentType string            `json:"contentType"`
	Payload     json.RawMessage   `json:"payload"`
}

// IsValid tellsthe mockbin is valid or not.
func (m Mockbin) IsValid() bool {
	return m.Status > 0
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

	if m.serveFn == nil {
		ep.createMockbin(m)
	}
	if m.serveFn == nil {
		ep.createEcho(m)
	}
	if m.serveFn == nil {
		ep.createProxy(m)
	}
	if m.serveFn == nil {
		ep.createDirect(m)
	}
	if m.serveFn == nil {
		ep.createDefault(m)
	}

	return m
}

func (ep *Endpoint) createDirect(m *APIDataModel) {
	direct := gjson.Get(ep.Body, "_direct")
	if direct.Type == gjson.Null {
		return
	}

	m.serveFn = func(c *gin.Context) {
		rsp := []byte(direct.String())
		c.Data(http.StatusOK, DetectContentType(rsp), rsp)
	}
}

func (ep *Endpoint) createDefault(m *APIDataModel) {
	dynamic := gjson.Get(ep.Body, "_dynamic")
	if dynamic.Type == gjson.JSON && HasPrefix(dynamic.Raw, "[") {
		m.dynamicValuers = createDynamics(ep.Body, []byte(dynamic.Raw))
	}

	model := *m
	m.serveFn = func(c *gin.Context) {
		if dynamicProcess(c, model) {
			return
		}

		b := []byte(ep.Body)
		c.Data(http.StatusOK, DetectContentType(b), b)
	}
}

func timeFmt(t time.Time) string {
	return t.Format("2006-01-02 15:04:05.0000")
}

func (ep *Endpoint) createMockbin(m *APIDataModel) {
	var mockbin Mockbin
	if err := json.Unmarshal([]byte(ep.Body), &mockbin); err != nil || !mockbin.IsValid() {
		return
	}

	m.serveFn = func(c *gin.Context) {
		if mockbin.Method != "" && mockbin.Method != c.Request.Method {
			c.Status(http.StatusMethodNotAllowed)
			return
		}

		for k, v := range mockbin.Headers {
			c.Header(k, v)
		}

		for _, v := range mockbin.Cookies {
			if v.Path == "" {
				v.Path = "/"
			}
			c.SetCookie(v.Name, v.Value, v.MaxAge, v.Path, v.Domain, v.Secure, v.HTTPOnly)
		}

		if mockbin.Close {
			c.Header("Connection", "close")
		}

		if mockbin.ContentType == "" {
			mockbin.ContentType = DetectContentType(mockbin.Payload)
		}

		c.Data(mockbin.Status, mockbin.ContentType, mockbin.Payload)
	}
}

func (ep *Endpoint) createEcho(m *APIDataModel) {
	echoType := gjson.Get(ep.Body, "_echo")
	if echoType.Type != gjson.String {
		return
	}

	echoMode := echoType.String()
	model := *m

	m.serveFn = func(c *gin.Context) {
		switch strings.ToLower(echoMode) {
		case "json":
			c.JSON(http.StatusOK, createRequestMap(c, model))
		default:
			dumpRequest, _ := httputil.DumpRequest(c.Request, true)
			c.Data(http.StatusOK, "text/plain; charset=utf-8", dumpRequest)
		}
	}
}

func createRequestMap(c *gin.Context, model APIDataModel) map[string]interface{} {
	r := c.Request
	m := map[string]interface{}{
		"timeGo":     timeFmt(time.Now()),
		"proto":      r.Proto,
		"host":       r.Host,
		"requestUri": r.RequestURI,
		"remoteAddr": r.RemoteAddr,
		"method":     r.Method,
		"url":        r.URL.String(),
		"headers":    convertHeader(r.Header),
	}

	fulfilRouter(c, model, m)
	fulfilQuery(r, m)
	fulfilOther(r, m)
	fulfilPayload(r, m)

	m["timeTo"] = timeFmt(time.Now())
	return m
}

func fulfilOther(r *http.Request, m map[string]interface{}) {
	if len(r.TransferEncoding) > 0 {
		m["transferEncoding"] = strings.Join(r.TransferEncoding, ",")
	}

	if r.Close {
		m["connection"] = "close"
	}
}

func fulfilRouter(c *gin.Context, model APIDataModel, m map[string]interface{}) {
	m["router"] = model.Endpoint
	if len(c.Params) > 0 {
		p := make(map[string]string)
		for _, pa := range c.Params {
			p[pa.Key] = pa.Value
		}

		m["routerParams"] = p
	}
}

func fulfilQuery(r *http.Request, m map[string]interface{}) {
	query := r.URL.Query()
	if len(query) > 0 {
		m["query"] = convertHeader(query)
	}
}

func convertHeader(query map[string][]string) map[string]string {
	q := make(map[string]string)
	for k, v := range query {
		q[k] = strings.Join(v, ", ")
	}

	return q
}

func fulfilPayload(r *http.Request, m map[string]interface{}) {
	payload, _ := ioutil.ReadAll(r.Body)
	if len(payload) > 0 {
		if HasContentType(r, "application/json") {
			m["payload"] = json.RawMessage(payload)
		} else {
			m["payload"] = string(payload)
		}
	}
}

// HasContentType determine whether the request `content-type` includes a
// server-acceptable mime-type
// Failure should yield an HTTP 415 (`http.StatusUnsupportedMediaType`)
func HasContentType(r *http.Request, mimetype string) bool {
	contentType := r.Header.Get("Content-type")
	if contentType == "" {
		return mimetype == "application/octet-stream"
	}

	for _, v := range strings.Split(contentType, ",") {
		if t, _, err := mime.ParseMediaType(v); err != nil {
			break
		} else if t == mimetype {
			return true
		}
	}

	return false
}

func (ep *Endpoint) createProxy(m *APIDataModel) {
	proxy := gjson.Get(ep.Body, "_proxy")
	isProxy := proxy.Type == gjson.String && HasPrefix(proxy.String(), "http")
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

	broadcastThrottler = MakeThrottle(60, 60*time.Second)
)

func compactJSON(data []byte) []byte {
	compactedBuffer := new(bytes.Buffer)
	if err := json.Compact(compactedBuffer, data); err != nil {
		v, _ := json.Marshal(map[string]string{"raw": string(data)})
		return v
	}

	return compactedBuffer.Bytes()
}

type routerResult struct {
	RouterServed   bool
	RouterBody     []byte
	Filename       string
	ResponseHeader map[string]string
	ResponseStatus int
	ResponseSize   int
	RemoteAddr     string
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

	for _, ep := range EndpointList(false) {
		path := JoinContextPath(ep.Endpoint)
		h := ep.handleJSON
		if ep.MimeType != "" {
			h = ep.handleFileDownload
		}

		if strings.EqualFold(ep.Method, "ANY") {
			router.Any(path, h)
		} else {
			router.Handle(ep.Method, path, h)
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
		h("Content-Disposition", mime.FormatMediaType("attachment",
			map[string]string{"filename": ep.Filename}))
		h("Content-Description", "File Transfer")
		h("Content-Type", "application/octet-stream")
		h("Content-Transfer-Encoding", "binary")
		h("Expires", "0")
		h("Cache-Control", "must-revalidate")
		h("Pragma", "public")
	}

	http.ServeContent(c.Writer, c.Request, ep.Filename, time.Now(),
		bytes.NewReader(ep.FileContent))
}

type writer struct {
	gin.ResponseWriter
	buf bytes.Buffer
}

func (w *writer) Write(data []byte) (n int, err error) {
	w.buf.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *writer) WriteString(s string) (n int, err error) {
	w.buf.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

func (w *writer) Body(maxSize int) string {
	if w.ResponseWriter.Size() <= maxSize {
		return w.buf.String()
	}

	return string(w.buf.Bytes()[:maxSize-3]) + "..."
}

func (ep APIDataModel) handleJSON(c *gin.Context) {
	if ep.serveFn != nil {
		copyWriter := &writer{
			ResponseWriter: c.Writer,
		}
		c.Writer = copyWriter
		ep.serveFn(c)

		routerResult := c.Request.Context().Value(routerResultKey).(*routerResult)
		if !routerResult.RouterServed {
			routerResult.RouterServed = true
			routerResult.RouterBody = copyWriter.buf.Bytes()
		}

		routerResult.RemoteAddr = c.Request.RemoteAddr
		routerResult.ResponseSize = copyWriter.Size()
		routerResult.ResponseStatus = copyWriter.Status()
		routerResult.ResponseHeader = convertHeader(copyWriter.Header())
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

func dynamicProcess(c *gin.Context, ep APIDataModel) bool {
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
		parameters := make(gin.H, len(v.parametersEvaluator))
		for k, valuer := range v.parametersEvaluator {
			parameters[k] = valuer(reqBody, c)
		}

		evaluateResult, err := v.expr.Evaluate(parameters)
		if err != nil {
			fmt.Println(err)
			return false
		}

		if yes, ok := evaluateResult.(bool); ok && yes {
			v.responseDynamic(c)

			return true
		}
	}

	return false
}

func (v dynamicValue) responseDynamic(c *gin.Context) {
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
		contentType = DetectContentType(v.Response)
	}

	c.Data(statusCode, contentType, v.Response)
}

func makeParameters(respBody string, expr *govaluate.EvaluableExpression) map[string]valuer {
	parameters := make(map[string]valuer)
	for _, va := range expr.Vars() {
		if HasPrefix(va, "json_") {
			k := va[5:]

			parameters[va] = func(reqBody []byte, c *gin.Context) interface{} {
				return gjson.GetBytes(reqBody, k).Value()
			}
		} else if HasPrefix(va, "query_") {
			k := va[6:]
			parameters[va] = func(reqBody []byte, c *gin.Context) interface{} {
				return c.Query(k)
			}
		} else if HasPrefix(va, "router_") {
			// /user/:user
			k := va[7:]
			parameters[va] = func(reqBody []byte, c *gin.Context) interface{} {
				return c.Param(k)
			}
		} else if HasPrefix(va, "header_") {
			k := va[7:]
			parameters[va] = func(reqBody []byte, c *gin.Context) interface{} {
				return c.GetHeader(k)
			}
		} else {
			indirectVa := gjson.Get(respBody, va).String()
			parameters[va] = func(reqBody []byte, c *gin.Context) interface{} {
				switch {
				case HasPrefix(indirectVa, "json:"):
					return gjson.GetBytes(reqBody, indirectVa[5:]).Value()
				case HasPrefix(indirectVa, "query:"):
					return c.Query(indirectVa[6:])
				case HasPrefix(indirectVa, "router:"):
					return c.Param(indirectVa[7:]) // /user/:user
				case HasPrefix(indirectVa, "header:"):
					return c.GetHeader(indirectVa[7:])
				default:
					return nil
				}
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

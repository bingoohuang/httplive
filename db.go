package httplive

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"net/http/httputil"
	"path"
	"strconv"
	"strings"
	"sync"
	textTemplate "text/template"
	"time"

	"github.com/julienschmidt/httprouter"

	"github.com/bingoohuang/jj"

	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/httplive/pkg/countable"

	"github.com/bingoohuang/gg/pkg/iox"

	"github.com/bingoohuang/gg/pkg/v"

	"github.com/asdine/storm/v3"
	"go.etcd.io/bbolt"

	"github.com/bingoohuang/golog/pkg/hlog"
	"github.com/bingoohuang/httplive/internal/process"
	"github.com/bingoohuang/httplive/pkg/http2curl"
	"github.com/bingoohuang/httplive/pkg/util"
	"github.com/gin-gonic/gin"

	"github.com/mssola/user_agent"
)

// Dao defines the api to access the database.
type Dao struct {
	db *storm.DB
}

// HasEndpoints tests if any endpoint exits already.
func (d *Dao) HasEndpoints() (has bool) {
	var result []process.Endpoint
	if err := d.db.All(&result, storm.Limit(1)); err != nil {
		log.Printf("ForEach error: %v", err)
	}

	return len(result) > 0
}

// ListEndpoints lists endpoints.
func (d *Dao) ListEndpoints() (result []process.Endpoint) {
	if err := d.db.All(&result); err != nil {
		log.Printf("ForEach error: %v", err)
	}
	return
}

// FindEndpoint finds endpoint with specified ID.
func (d *Dao) FindEndpoint(ID uint64) *process.Endpoint {
	result := &process.Endpoint{}
	err := d.db.One("ID", ID, result)
	if err == storm.ErrNotFound {
		return nil
	}
	if err != nil {
		log.Printf("find error: %v", err)
	}
	return result
}

// FindByEndpoint finds endpoint by its value.
func (d *Dao) FindByEndpoint(endpoint string) *process.Endpoint {
	result := &process.Endpoint{}
	err := d.db.One("Endpoint", endpoint, result)
	if err == storm.ErrNotFound {
		return nil
	}
	if err != nil {
		log.Printf("find error: %v", err)
	}

	return result
}

// AddEndpoint adds a endpoint.
func (d *Dao) AddEndpoint(ep process.Endpoint) uint64 {
	if err := d.db.Save(&ep); err != nil {
		log.Printf("insert error: %v", err)
	}

	return ep.ID
}

// UpdateEndpoint updates a endpoint.
func (d *Dao) UpdateEndpoint(ep process.Endpoint) {
	if err := d.db.Update(&ep); err != nil {
		log.Printf("Update error: %v", err)
	}
}

// DeleteEndpoint delete a endpoint.
func (d *Dao) DeleteEndpoint(ep process.Endpoint) {
	if err := d.db.DeleteStruct(&ep); err != nil {
		log.Printf("Delete error: %v", err)
	}
}

// Backup backups a bolt db file.
func (d *Dao) Backup(w http.ResponseWriter, name string) {
	err := d.db.Bolt.View(func(tx *bbolt.Tx) error {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
		w.Header().Set("Content-Length", strconv.Itoa(int(tx.Size())))
		_, err := tx.WriteTo(w)
		return err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// CreateDao creates a dao.
func CreateDao(db *storm.DB) (*Dao, error) {
	return &Dao{db: db}, nil
}

var (
	//go:embed assets
	assetsFS  embed.FS
	subAssets fs.FS
)

func init() {
	subAssets, _ = fs.Sub(assetsFS, "assets")
	process.DirListTemplate = func() *template.Template {
		t, err := template.New("dirlist").
			Funcs(template.FuncMap{
				"t2s": timeToString,
			}).
			Parse(asset("dirlist.html"))
		if err != nil {
			log.Fatalf("Directory list template init error: %v", err)
		}
		return t
	}()

	process.EchartsTemplate = func() *textTemplate.Template {
		t, err := textTemplate.New("echarts").
			Funcs(textTemplate.FuncMap{
				"ToString":  toString,
				"JoinLines": joinLiteralLines,
			}).
			Parse(asset("echarts.html"))
		if err != nil {
			log.Fatalf("Directory echarts template init error: %v", err)
		}
		return t
	}()
}

func joinLiteralLines(lines []string) string {
	return strings.Join(lines, `\n`)
}

func toString(value interface{}) string {
	switch val := value.(type) {
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	case bool:
		return fmt.Sprintf("%t", val)
	case json.RawMessage:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func asset(name string) string {
	data, err := fs.ReadFile(subAssets, name)
	if err != nil {
		panic(err)
	}

	return string(data)
}

func timeToString(t time.Time, format ...string) string {
	f := "2006-01-02 15:04:05"
	if len(format) > 0 && format[0] != "" {
		f = format[0]
	}
	return t.Format(f)
}

var dbLock sync.Mutex

// DBDo executes the f.
func DBDo(f func(dao *Dao) error) error {
	dbLock.Lock()
	defer dbLock.Unlock()

	db, err := storm.Open(Envs.DBFile)
	defer iox.Close(db)

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

	SyncAPIRouter()
	return nil
}

func createDB(dao *Dao) error {
	if dao.HasEndpoints() {
		return nil
	}

	now := util.TimeFmt(time.Now())
	dao.AddEndpoint(process.Endpoint{ID: 0, Endpoint: "/auth/demo", Methods: http.MethodGet, MimeType: "", Filename: "", Body: asset("auth.json"), CreateTime: now, UpdateTime: now, DeletedAt: ""})
	dao.AddEndpoint(process.Endpoint{ID: 0, Endpoint: "/api/demo", Methods: http.MethodGet, MimeType: "", Filename: "", Body: asset("apidemo.json"), CreateTime: now, UpdateTime: now, DeletedAt: ""})
	dao.AddEndpoint(process.Endpoint{ID: 0, Endpoint: "/dynamic/demo", Methods: http.MethodPost, MimeType: "", Filename: "", Body: asset("dynamicdemo.json"), CreateTime: now, UpdateTime: now, DeletedAt: ""})
	dao.AddEndpoint(process.Endpoint{ID: 0, Endpoint: "/proxy/demo", Methods: http.MethodGet, MimeType: "", Filename: "", Body: asset("proxydemo.json"), CreateTime: now, UpdateTime: now, DeletedAt: ""})
	dao.AddEndpoint(process.Endpoint{ID: 0, Endpoint: "/echo/:id", Methods: "ANY", MimeType: "", Filename: "", Body: asset("echo.json"), CreateTime: now, UpdateTime: now, DeletedAt: ""})
	dao.AddEndpoint(process.Endpoint{ID: 0, Endpoint: "/mockbin", Methods: "ANY", MimeType: "", Filename: "", Body: asset("mockbin.json"), CreateTime: now, UpdateTime: now, DeletedAt: ""})
	dao.AddEndpoint(process.Endpoint{ID: 0, Endpoint: "/serveStatic", Methods: http.MethodGet, MimeType: "", Filename: "", Body: asset("servestatic.json"), CreateTime: now, UpdateTime: now, DeletedAt: ""})
	dao.AddEndpoint(process.Endpoint{ID: 0, Endpoint: "/sqli", Methods: "ANY", MimeType: "", Filename: "", Body: asset("sqli.json"), CreateTime: now, UpdateTime: now, DeletedAt: ""})
	dao.AddEndpoint(process.Endpoint{ID: 0, Endpoint: "/form", Methods: "ANY", MimeType: "", Filename: "", Body: asset("form.json"), CreateTime: now, UpdateTime: now, DeletedAt: ""})
	dao.AddEndpoint(process.Endpoint{ID: 0, Endpoint: "/upload", Methods: "ANY", MimeType: "", Filename: "", Body: asset("upload.json"), CreateTime: now, UpdateTime: now, DeletedAt: ""})
	dao.AddEndpoint(process.Endpoint{ID: 0, Endpoint: "/echarts1", Methods: http.MethodGet, MimeType: "", Filename: "", Body: asset("echarts1.json"), CreateTime: now, UpdateTime: now, DeletedAt: ""})
	dao.AddEndpoint(process.Endpoint{ID: 0, Endpoint: "/echarts2", Methods: http.MethodGet, MimeType: "", Filename: "", Body: asset("echarts2.json"), CreateTime: now, UpdateTime: now, DeletedAt: ""})
	dao.AddEndpoint(process.Endpoint{ID: 0, Endpoint: "/echarts3", Methods: http.MethodGet, MimeType: "", Filename: "", Body: asset("echarts3.json"), CreateTime: now, UpdateTime: now, DeletedAt: ""})
	dao.AddEndpoint(process.Endpoint{ID: 0, Endpoint: "/eval", Methods: "ANY", MimeType: "", Filename: "", Body: asset("evaldemo.json"), CreateTime: now, UpdateTime: now, DeletedAt: ""})
	dao.AddEndpoint(process.Endpoint{ID: 0, Endpoint: "/health", Methods: http.MethodGet, MimeType: "", Filename: "", Body: `{"Status": "OK"}`, CreateTime: now, UpdateTime: now, DeletedAt: ""})
	dao.AddEndpoint(process.Endpoint{ID: 0, Endpoint: "/status", Methods: http.MethodGet, MimeType: "", Filename: "", Body: `{"Status": "OK"}`, CreateTime: now, UpdateTime: now, DeletedAt: ""})
	// dao.AddEndpointID(process.Endpoint{ ID: f(), Endpoint: "/_internal/apiacl", Methods: "ANY", MimeType: "", Filename: "", Body: asset("apiacl.casbin"), CreateTime: now, UpdateTime: now, DeletedAt: "", })
	// dao.AddEndpointID(process.Endpoint{ ID: f(), Endpoint: "/_internal/adminacl", Methods: "ANY", MimeType: "", Filename: "", Body: asset("adminacl.casbin"), CreateTime: now, UpdateTime: now, DeletedAt: "", })

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
		old := dao.FindEndpoint(model.ID.Int())
		if old == nil {
			old = dao.FindByEndpoint(model.Endpoint)
		}

		bean := CreateEndpoint(model, old)

		if old == nil {
			bean.ID = dao.AddEndpoint(bean)
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
		ID:          process.ID(fmt.Sprintf("%d", ep.ID)),
		Endpoint:    ep.Endpoint,
		Method:      ep.Methods,
		MimeType:    ep.MimeType,
		Filename:    ep.Filename,
		FileContent: ep.FileContent,
		Body:        ep.Body,
	}

	if query {
		return m
	}

	m.TryDo(ep.CreateHlHandlers, asset)
	m.TryDo(ep.CreateEcho, nil)
	m.TryDo(ep.CreateProxy, nil)
	m.TryDo(ep.CreateDirect, nil)
	m.TryDo(ep.CreateDefault, nil)

	return m
}

// CreateEndpoint creates an endpoint from APIDataModel.
func CreateEndpoint(model process.APIDataModel, old *process.Endpoint) process.Endpoint {
	now := util.TimeFmt(time.Now())

	ep := process.Endpoint{
		ID: model.ID.Int(), Endpoint: model.Endpoint, Methods: model.Method, MimeType: model.MimeType,
		Filename: model.Filename, FileContent: model.FileContent,
		Body: model.Body, CreateTime: now, UpdateTime: now, DeletedAt: "",
	}
	if old != nil {
		if old.Body != "" && ep.Body == "" {
			ep.Body = old.Body
		}

		if old.ID != 0 && ep.ID == 0 {
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
			ID:        process.ID(id).Int(),
			DeletedAt: util.TimeFmt(time.Now()),
		})

		return nil
	})
}

// GetByEndpoint ...
func GetByEndpoint(endpoint string) (*process.APIDataModel, error) {
	var model *process.APIDataModel

	err := DBDo(func(dao *Dao) error {
		ep := dao.FindByEndpoint(endpoint)
		model = CreateAPIDataModel(ep, true)

		return nil
	})

	return model, err
}

// GetEndpoint ...
func GetEndpoint(id process.ID) (*process.APIDataModel, error) {
	var model *process.APIDataModel

	err := DBDo(func(dao *Dao) error {
		ep := dao.FindEndpoint(id.Int())
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
func JoinContextPath(elem string, p *process.APIDataModel) string {
	if p != nil {
		if h := jj.Get(p.Body, "_hl"); h.String() == process.HlServerStatic {
			if prefix, hasParams := process.ParsePathParams(p); !hasParams {
				elem = path.Join(prefix, "/*file")
			}
		}
	}

	if Envs.ContextPath == "/" {
		return elem
	}

	return path.Join(Envs.ContextPath, elem)
}

// TestAPIRouter ...
func TestAPIRouter(p process.APIDataModel) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(r.(string))
		}
	}()

	router := httprouter.New()
	for _, ep := range EndpointList(false) {
		if ep.ID != p.ID {
			contextPath := JoinContextPath(ep.Endpoint, &ep)
			router.GET(contextPath, nil)
		}
	}

	contextPath := JoinContextPath(p.Endpoint, &p)
	router.GET(contextPath, nil)
	return nil
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
	endpoint := ep.Endpoint
	if strings.HasPrefix(endpoint, "/_internal") {
		ep.InternalProcess(endpoint[10:])
		return
	}

	h := ep.HandleJSON
	if ep.MimeType != "" {
		h = ep.HandleFileDownload
	}

	if strings.EqualFold(ep.Method, "ANY") {
		r.Any(JoinContextPath(endpoint, &ep), h)
	} else {
		r.Handle(ep.Method, JoinContextPath(endpoint, &ep), h)
	}
}

func noRouteHandlerWrap(c *gin.Context) {
	cw := util.NewGinCopyWriter(c.Writer, c)
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

var counter countable.Counter

func noRouteHandler(c *gin.Context) (processed bool) {
	processed = true
	p := trimContextPath(c)

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
		c.IndentedJSON(http.StatusOK, gin.H{"version": v.AppVersion, "build": v.BuildTime, "go": v.GoVersion, "git": v.GitCommit})
	case hl == "curl" || p == "/curl":
		values := c.Request.URL.Query()
		delete(values, "_hl")
		c.Request.URL.RawQuery = values.Encode()
		cmd, _ := http2curl.GetCurlCmd(c.Request)
		c.Data(http.StatusOK, util.ContentTypeText, []byte(cmd.String()))
	case hl == "counter" || p == "/counter":
		c.IndentedJSON(http.StatusOK, counterDeal(c.Query))
	case hl == "ip" || p == "/ip":
		process.ProcessIP(c, useJSON)
	case hl == "time" || p == "/time":
		if useJSON {
			c.IndentedJSON(http.StatusOK, gin.H{"time": util.TimeFmt(time.Now())})
		} else {
			c.Data(http.StatusOK, util.ContentTypeText, []byte(util.TimeFmt(time.Now())))
		}
	case (hl == "" && p == "/") || hl == "echo" || p == "/echo":
		if useJSON {
			c.IndentedJSON(http.StatusOK, process.CreateRequestMap(c, nil))
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

func multiQuery(query func(key string) string, keys ...string) string {
	for _, k := range keys {
		if value := query(k); value != "" {
			return value
		}
	}

	return ""
}

func counterDeal(query func(key string) string) gin.H {
	key := strings.ToLower(multiQuery(query, "key", "k"))
	key = ss.Or(key, "default")
	switch op := strings.ToLower(query("op")); op {
	case "increment", "incr", "inc", "i":
		value := int64(1)
		if val, err := ss.ParseInt64E(multiQuery(query, "value", "val", "v")); err == nil {
			value = val
		}
		return gin.H{"counter": counter.Add(key, value)}
	case "deduct", "dede", "ded", "d":
		value := int64(-1)
		if val, err := ss.ParseInt64E(multiQuery(query, "value", "val", "v")); err == nil {
			value = val
		}
		return gin.H{"counter": counter.Add(key, value)}
	case "all", "a":
		h := gin.H{}
		counter.Range(func(key string, value int64) bool {
			h[key] = value
			return true
		})
		return gin.H{"counter": h}
	case "query", "q":
		return gin.H{"counter": counter.GetValue(key)}
	case "reset", "r", "delete", "del":
		lastValue, loaded := counter.DeleteAndGetLastValue(key)
		return gin.H{"counter": 0, "last": lastValue, "loaded": loaded}
	default:
		return gin.H{"counter": counter.Add(key, 1)}
	}
}

// EndpointList ...
func EndpointList(query bool) []process.APIDataModel {
	var endPoints []process.Endpoint

	_ = DBDo(func(dao *Dao) error {
		endPoints = dao.ListEndpoints()
		return nil
	})

	items := make([]process.APIDataModel, len(endPoints))
	for i, val := range endPoints {
		val := val
		items[i] = *CreateAPIDataModel(&val, query)
	}

	return items
}

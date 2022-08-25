package process

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/ast"
	"github.com/antonmedv/expr/parser"
	"github.com/mssola/user_agent"

	"github.com/bingoohuang/httplive/pkg/eval"
	"github.com/bingoohuang/httplive/pkg/httptee"
	"github.com/bingoohuang/httplive/pkg/lb"
	"github.com/bingoohuang/httplive/pkg/util"
	"github.com/bingoohuang/jj"
	"github.com/gin-gonic/gin"
)

// RouterResult result for router
type RouterResult struct {
	RouterServed   bool
	RouterBody     []byte
	Filename       string
	ResponseHeader map[string]string
	ResponseStatus int
	ResponseSize   int
	RemoteAddr     string
}

func (ep Endpoint) CreateProxy(m *APIDataModel, _ func(name string) string) {
	proxy := jj.Get(ep.Body, "_proxy")
	isProxy := proxy.Type == jj.String && util.HasPrefix(proxy.String(), "http")
	if !isProxy {
		return
	}

	pool := lb.CreateProxyServerPool(proxy.String(), ep.Methods+" "+ep.Endpoint)
	if err := pool.CheckBackends(); err != nil {
		log.Printf("E! proxy server check failed %v", err)
		return
	}

	var (
		err        error
		teeHandler *httptee.Handler
	)

	if isTee := proxy.Type == jj.String && util.HasPrefix(proxy.String(), "http"); isTee {
		tee := jj.Get(ep.Body, "_tee")
		if teeHandler, err = httptee.CreateHandler(tee.String()); err != nil {
			log.Printf("E! tee server failed %v", err)
		}
	}

	m.ServeFn = func(c *gin.Context) {
		if teeHandler != nil {
			teeHandler.Tee(c.Request)
		}

		p := pool.GetNextPeer()
		rp := util.ReverseProxy(c.Request.URL.String(), p.Addr.Host, p.Addr.Path)
		rp.ServeHTTP(c.Writer, c.Request)
	}
}

func (ep *Endpoint) CreateDirect(m *APIDataModel, _ func(name string) string) {
	direct := jj.Get(ep.Body, "_direct")
	if direct.Type == jj.Null {
		return
	}

	_, authBean := ParseAuth(ep.Body)

	m.ServeFn = func(c *gin.Context) {
		if !authBean.AuthRequest(c) {
			return
		}

		util.GinData(c, []byte(eval.JjGen(direct.String())))
	}
}

func (ep *Endpoint) CreateDefault(m *APIDataModel, _ func(name string) string) {
	body := ep.Body
	dynamic := jj.Get(body, "_dynamic")
	if dynamic.Type == jj.JSON && dynamic.IsArray() {
		m.dynamicValuers = createDynamics(body, []byte(dynamic.Raw))
	}

	model := *m

	body, authBean := ParseAuth(ep.Body)
	body, _ = jj.Delete(body, "_hl")
	body, _ = jj.Delete(body, "_dynamic")

	m.ServeFn = func(c *gin.Context) {
		if !authBean.AuthRequest(c) {
			return
		}

		if dynamicProcess(c, model) {
			return
		}

		util.GinData(c, []byte(Eval(ep.Endpoint, body)))
	}
}

type HlHandler interface {
	HlHandle(c *gin.Context, apiModel *APIDataModel, asset func(name string) string) error
}

type HlHandlerFn func(c *gin.Context, apiModel *APIDataModel) error

func (f HlHandlerFn) HlHandle(c *gin.Context, apiModel *APIDataModel) error {
	return f(c, apiModel)
}

type HlHandlerCreator func() HlHandler

var hlHandlers = map[string]HlHandlerCreator{}

func registerHlHandlers(k string, creator HlHandlerCreator) {
	hlHandlers[k] = creator
}

func (ep *Endpoint) CreateHlHandlers(m *APIDataModel, asset func(name string) string) {
	for k, v := range hlHandlers {
		if h := jj.Get(ep.Body, "_hl"); h.String() == k {
			if ep.CreateHlHandler(m, asset, v) {
				return
			}
		}
	}
}

func (ep *Endpoint) CreateHlHandler(m *APIDataModel, asset func(name string) string, v HlHandlerCreator) bool {
	b := v()
	if err := json.Unmarshal([]byte(ep.Body), b); err != nil {
		return true
	}
	if bb, ok := b.(AfterUnmashaler); ok {
		bb.AfterUnmashal()
	}

	_, authBean := ParseAuth(ep.Body)

	m.ServeFn = func(ctx *gin.Context) {
		if !authBean.AuthRequest(ctx) {
			return
		}

		if a, ok := b.(MethodsAllowed); ok {
			if !a.AllowMethods(ctx.Request.Method) {
				ctx.Status(http.StatusMethodNotAllowed)
				return
			}
		}

		if err := b.HlHandle(ctx, m, asset); err != nil {
			log.Printf("E! %v", err)
			_ = ctx.AbortWithError(http.StatusInternalServerError, err)
		}
	}
	return false
}

func (ep *Endpoint) CreateEcho(m *APIDataModel, _ func(name string) string) {
	echoType := jj.Get(ep.Body, "_echo")
	if echoType.Type != jj.String {
		return
	}

	echoMode := echoType.String()
	model := *m

	m.ServeFn = func(c *gin.Context) {
		switch strings.ToLower(echoMode) {
		case "json":
			c.IndentedJSON(http.StatusOK, CreateRequestMap(c, &model))
		default:
			dumpRequest, _ := httputil.DumpRequest(c.Request, true)
			c.Data(http.StatusOK, util.ContentTypeText, dumpRequest)
		}
	}
}

func CreateRequestMap(c *gin.Context, model *APIDataModel) map[string]interface{} {
	r := c.Request
	m := map[string]interface{}{
		"timeGo":     util.TimeFmt(time.Now()),
		"proto":      r.Proto,
		"host":       r.Host,
		"requestUri": r.RequestURI,
		"remoteAddr": r.RemoteAddr,
		"method":     r.Method,
		"url":        r.URL.String(),
		"headers":    util.ConvertHeader(r.Header),
	}

	if model != nil {
		fulfilRouter(c, model, m)
	}
	fulfilQuery(r, m)
	fulfilUserAgent(r, m)
	fulfilOther(r, m)
	fulfilPayload(r, m, c.Query("body"))

	m["timeTo"] = util.TimeFmt(time.Now())
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

func fulfilRouter(c *gin.Context, model *APIDataModel, m map[string]interface{}) {
	m["router"] = model.Endpoint
	if len(c.Params) > 0 {
		p := make(map[string]string)
		for _, pa := range c.Params {
			p[pa.Key] = pa.Value
		}

		m["routerParams"] = p
	}
}

func fulfilUserAgent(r *http.Request, m map[string]interface{}) {
	userAgent := r.UserAgent()
	ua := user_agent.New(userAgent)
	m["Ua-OS"] = ua.OS()
	browser, browserVersion := ua.Browser()
	m["Ua-Browser"] = browser
	m["Ua-BrowserVersion"] = browserVersion
	m["Ua-Bot"] = ua.Bot()
	m["Ua-Mobile"] = ua.Mobile()
	engine, engineVersion := ua.Engine()
	m["Ua-Engine"] = engine
	m["Ua-EngineVersion"] = engineVersion
	m["Ua-Mozilla"] = ua.Mozilla()
	m["Ua-OSInfo"] = ua.OSInfo()
	m["Ua-Platform"] = ua.Platform()
	m["Ua-Localization"] = ua.Localization()
	m["Ua-OS"] = ua.OS()
}

func fulfilQuery(r *http.Request, m map[string]interface{}) {
	if query := r.URL.Query(); len(query) > 0 {
		m["query"] = util.ConvertHeader(query)
	}
}

func fulfilPayload(r *http.Request, m map[string]interface{}, body string) {
	if body == "no" {
		return
	}

	if p, _ := io.ReadAll(r.Body); len(p) > 0 {
		typ, outi, ok := jj.ValidPayload(p, 0)
		if ok && typ == jj.JSON && len(p[outi:]) == 0 {
			m["payload"] = json.RawMessage(p)
		} else {
			m["payload"] = string(p)
		}
	}
}

type visitor struct {
	identifiers []string
}

func (v *visitor) Enter(_ *ast.Node) {}
func (v *visitor) Exit(node *ast.Node) {
	if n, ok := (*node).(*ast.IdentifierNode); ok {
		v.identifiers = append(v.identifiers, n.Value)
	}
}

func createDynamics(epBody string, dynamicRaw []byte) (dynamicValues []DynamicValue) {
	if err := json.Unmarshal(dynamicRaw, &dynamicValues); err != nil {
		fmt.Println(err)
		return
	}

	for i, v := range dynamicValues {
		tree, err := parser.Parse(v.Condition)
		if err != nil {
			fmt.Println(err)
			return
		}

		exp, err := expr.Compile(v.Condition)
		if err != nil {
			fmt.Println(err)
			return
		}

		vi := &visitor{}
		ast.Walk(&tree.Node, vi)

		dynamicValues[i].Expr = exp
		dynamicValues[i].ParametersEvaluator = MakeParamValuer(epBody, vi.identifiers)
	}

	return
}

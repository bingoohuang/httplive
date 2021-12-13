package process

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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

func (ep Endpoint) CreateProxy(m *APIDataModel) {
	proxy := jj.Get(ep.Body, "_proxy")
	isProxy := proxy.Type == jj.String && util.HasPrefix(proxy.String(), "http")
	if !isProxy {
		return
	}

	pool := lb.CreateProxyServerPool(proxy.String())
	if err := pool.CheckBackends(); err != nil {
		log.Printf("E! proxy server check failed %v", err)
		return
	}

	var (
		err            error
		httpteeHandler *httptee.Handler
	)

	if isTee := proxy.Type == jj.String && util.HasPrefix(proxy.String(), "http"); isTee {
		tee := jj.Get(ep.Body, "_tee")
		if httpteeHandler, err = httptee.CreateHandler(tee.String()); err != nil {
			log.Printf("E! tee server failed %v", err)
		}
	}

	m.ServeFn = func(c *gin.Context) {
		if httpteeHandler != nil {
			httpteeHandler.Tee(c.Request)
		}

		p := pool.GetNextPeer()
		rp := util.ReverseProxy(c.Request.URL.String(), p.Addr.Host, p.Addr.Path)
		rp.ServeHTTP(c.Writer, c.Request)
	}
}

func (ep *Endpoint) CreateDirect(m *APIDataModel) {
	direct := jj.Get(ep.Body, "_direct")
	if direct.Type == jj.Null {
		return
	}

	m.ServeFn = func(c *gin.Context) {
		util.GinData(c, []byte(eval.JjGen(direct.String())))
	}
}

func (ep *Endpoint) CreateDefault(m *APIDataModel) {
	dynamic := jj.Get(ep.Body, "_dynamic")
	if dynamic.Type == jj.JSON && dynamic.IsArray() {
		m.dynamicValuers = createDynamics(ep.Body, []byte(dynamic.Raw))
	}

	model := *m
	m.ServeFn = func(c *gin.Context) {
		if dynamicProcess(c, model) {
			return
		}

		util.GinData(c, []byte(eval.Eval(ep.Endpoint, ep.Body)))
	}
}

func (ep *Endpoint) CreateMockbin(m *APIDataModel) {
	echoType := jj.Get(ep.Body, "_mockbin")
	if !echoType.Bool() {
		return
	}

	var b Mockbin

	if err := json.Unmarshal([]byte(ep.Body), &b); err != nil || !b.IsValid() {
		return
	}

	m.ServeFn = b.Handle
}

func (ep *Endpoint) CreateEcho(m *APIDataModel) {
	echoType := jj.Get(ep.Body, "_echo")
	if echoType.Type != jj.String {
		return
	}

	echoMode := echoType.String()
	model := *m

	m.ServeFn = func(c *gin.Context) {
		switch strings.ToLower(echoMode) {
		case "json":
			c.PureJSON(http.StatusOK, CreateRequestMap(c, &model))
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
	fulfilPayload(r, m)

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
	m["User-Agent-OS"] = ua.OS()
	browser, browserVersion := ua.Browser()
	m["User-Agent-Browser"] = browser
	m["User-Agent-BrowserVersion"] = browserVersion
	m["User-Agent-Bot"] = ua.Bot()
	m["User-Agent-Mobile"] = ua.Mobile()
	engine, engineVersion := ua.Engine()
	m["User-Agent-Engine"] = engine
	m["User-Agent-EngineVersion"] = engineVersion
	m["User-Agent-Mozilla"] = ua.Mozilla()
	m["User-Agent-OSInfo"] = ua.OSInfo()
	m["User-Agent-Platform"] = ua.Platform()
	m["User-Agent-Localization"] = ua.Localization()
	m["User-Agent-OS"] = ua.OS()
}

func fulfilQuery(r *http.Request, m map[string]interface{}) {
	if query := r.URL.Query(); len(query) > 0 {
		m["query"] = util.ConvertHeader(query)
	}
}

func fulfilPayload(r *http.Request, m map[string]interface{}) {
	payload, _ := ioutil.ReadAll(r.Body)
	if len(payload) > 0 {
		if util.HasContentType(r, "application/json") {
			m["payload"] = json.RawMessage(payload)
		} else {
			m["payload"] = string(payload)
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

		expr, err := expr.Compile(v.Condition)
		if err != nil {
			fmt.Println(err)
			return
		}

		visitor := &visitor{}
		ast.Walk(&tree.Node, visitor)

		dynamicValues[i].Expr = expr
		dynamicValues[i].ParametersEvaluator = MakeParamValuer(epBody, visitor.identifiers)
	}

	return
}

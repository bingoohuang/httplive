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

	"github.com/bingoohuang/jj"

	"github.com/bingoohuang/httplive/pkg/eval"

	"github.com/bingoohuang/httplive/pkg/httptee"

	"github.com/bingoohuang/httplive/pkg/lb"

	"github.com/bingoohuang/govaluate"
	"github.com/bingoohuang/httplive/pkg/util"
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

	tee := jj.Get(ep.Body, "_tee")
	isTee := proxy.Type == jj.String && util.HasPrefix(proxy.String(), "http")
	if isTee {
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

		dynamicValues[i].Expr = expr
		dynamicValues[i].ParametersEvaluator = MakeParamValuer(epBody, expr)
	}

	return
}

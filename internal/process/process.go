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

	"github.com/bingoohuang/httplive/pkg/lb"

	"github.com/Knetic/govaluate"
	"github.com/bingoohuang/httplive/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
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
	proxy := gjson.Get(ep.Body, "_proxy")
	isProxy := proxy.Type == gjson.String && util.HasPrefix(proxy.String(), "http")
	if !isProxy {
		return
	}

	pool := lb.CreateServerPool(proxy.String())
	if err := pool.CheckBackends(); err != nil {
		log.Printf("E! proxy server check failed %v", err)
		return
	}

	m.ServeFn = func(c *gin.Context) {
		p := pool.GetNextPeer()
		rp := util.ReverseProxy(c.Request.URL.String(), p.Addr.Host, p.Addr.Path)
		rp.ServeHTTP(c.Writer, c.Request)
	}
}

func (ep *Endpoint) CreateDirect(m *APIDataModel) {
	direct := gjson.Get(ep.Body, "_direct")
	if direct.Type == gjson.Null {
		return
	}

	m.ServeFn = func(c *gin.Context) {
		util.GinData(c, []byte(direct.String()))
	}
}

func (ep *Endpoint) CreateDefault(m *APIDataModel) {
	dynamic := gjson.Get(ep.Body, "_dynamic")
	if dynamic.Type == gjson.JSON && dynamic.IsArray() {
		m.dynamicValuers = createDynamics(ep.Body, []byte(dynamic.Raw))
	}

	model := *m
	m.ServeFn = func(c *gin.Context) {
		if dynamicProcess(c, model) {
			return
		}

		util.GinData(c, []byte(ep.Body))
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
	echoType := gjson.Get(ep.Body, "_echo")
	if echoType.Type != gjson.String {
		return
	}

	echoMode := echoType.String()
	model := *m

	m.ServeFn = func(c *gin.Context) {
		switch strings.ToLower(echoMode) {
		case "json":
			c.JSON(http.StatusOK, createRequestMap(c, model))
		default:
			dumpRequest, _ := httputil.DumpRequest(c.Request, true)
			c.Data(http.StatusOK, util.ContentTypeText, dumpRequest)
		}
	}
}

func createRequestMap(c *gin.Context, model APIDataModel) map[string]interface{} {
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

	fulfilRouter(c, model, m)
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

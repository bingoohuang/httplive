package process

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/antonmedv/expr/vm"
	"github.com/bingoohuang/httplive/pkg/eval"
	"github.com/bingoohuang/httplive/pkg/util"
	"github.com/bingoohuang/jj"
	"github.com/gin-gonic/gin"
)

type Valuer func(reqBody []byte, c *gin.Context) interface{}

// DynamicValue works for dynamic processing.
type DynamicValue struct {
	Headers map[string]string `json:"headers"`

	Expr                *vm.Program
	ParametersEvaluator map[string]Valuer
	Condition           string          `json:"condition"`
	Response            json.RawMessage `json:"response"`
	Status              int             `json:"status"`
}

func (v DynamicValue) responseDynamic(ep APIDataModel, c *gin.Context) {
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

	payload := []byte(Eval(ep.Endpoint, string(v.Response)))
	if contentType == "" {
		contentType = util.DetectContentType(payload)
	}

	c.Data(statusCode, contentType, payload)
}

func Eval(endpoint string, body string) string {
	return eval.JjGen(eval.Execute(endpoint, body))
}

func MakeParamValuer(jsonConfig string, vars []string) map[string]Valuer {
	parameters := make(map[string]Valuer)
	for _, va := range vars {
		parameters[va] = makeParameter(va, jsonConfig)
	}

	return parameters
}

func makeParameter(va string, jsonConfig string) Valuer {
	switch {
	case util.HasPrefix(va, "json_"):
		k := va[5:]
		return func(payload []byte, c *gin.Context) interface{} { return jj.GetBytes(payload, k).Value() }
	case util.HasPrefix(va, "query_"):
		k := va[6:]
		return func(_ []byte, c *gin.Context) interface{} { return c.Query(k) }
	case util.HasPrefix(va, "router_"):
		// /user/:user
		k := va[7:]
		return func(_ []byte, c *gin.Context) interface{} { return c.Param(k) }
	case util.HasPrefix(va, "header_"):
		k := va[7:]
		return func(_ []byte, c *gin.Context) interface{} { return c.GetHeader(k) }
	default:
		indirectVa := jj.Get(jsonConfig, va).String()
		if indirectVa == "" {
			return func(_ []byte, c *gin.Context) interface{} { return nil }
		}

		return makeParameter(indirectVa, jsonConfig)
	}
}

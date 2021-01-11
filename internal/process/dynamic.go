package process

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bingoohuang/httplive/pkg/eval"

	"github.com/bingoohuang/govaluate"
	"github.com/bingoohuang/httplive/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type Valuer func(reqBody []byte, c *gin.Context) interface{}

// DynamicValue works for dynamic processing.
type DynamicValue struct {
	Condition string            `json:"condition"`
	Response  json.RawMessage   `json:"response"`
	Status    int               `json:"status"`
	Headers   map[string]string `json:"headers"`

	Expr                *govaluate.EvaluableExpression
	ParametersEvaluator map[string]Valuer
}

func (v DynamicValue) responseDynamic(c *gin.Context) {
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

	payload := []byte(eval.Eval(string(v.Response)))
	if contentType == "" {
		contentType = util.DetectContentType(payload)
	}

	c.Data(statusCode, contentType, payload)
}

func MakeParamValuer(jsonConfig string, expr *govaluate.EvaluableExpression) map[string]Valuer {
	parameters := make(map[string]Valuer)
	for _, va := range expr.Vars() {
		parameters[va] = makeParameter(va, jsonConfig)
	}

	return parameters
}

func makeParameter(va string, jsonConfig string) Valuer {
	switch {
	case util.HasPrefix(va, "json_"):
		k := va[5:]
		return func(payload []byte, c *gin.Context) interface{} { return gjson.GetBytes(payload, k).Value() }
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
		indirectVa := gjson.Get(jsonConfig, va).String()
		if indirectVa == "" {
			return func(_ []byte, c *gin.Context) interface{} { return nil }
		}

		return makeParameter(indirectVa, jsonConfig)
	}
}

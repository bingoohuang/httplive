package util

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bingoohuang/jj"

	"github.com/bingoohuang/govaluate"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestUnquoteCover(t *testing.T) {
	assert.Equal(t, "abc",
		UnquoteCover("=start=abc=end=", "=start=", "=end="))
}

func TestCreateEndpointKey(t *testing.T) {
	tests := []struct {
		method   string
		endpoint string
		out      string
	}{
		{"", "", ""},
		{http.MethodPost, "foo", "postfoo"},
		{http.MethodPost, "FOO", "postfoo"},
		{http.MethodPost, "foo", "postfoo"},
		{http.MethodPost, "FOO", "postfoo"},
		{"ÄËÏ", "ÖÜ", "äëïöü"},
		{http.MethodPost, "///", "post///"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.out, JoinLowerKeys(tt.method, tt.endpoint))
	}
}

type DynamicValue struct {
	Condition string          `json:"condition"`
	Response  json.RawMessage `json:"response"`
}

func TestGson(t *testing.T) {
	j := []byte(`
{
  "_dynamic": [
    {
      "condition":"json_name == 'bingoo'",
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
    }
  ]
}
`)

	result := jj.GetBytes(j, "_dynamic")
	fmt.Println(result)

	reqBody := []byte(`
{
	"name":"bingoo"
}
`)

	if result.Type == jj.JSON {
		var dynamicValues []DynamicValue
		if err := json.Unmarshal([]byte(result.Raw), &dynamicValues); err != nil {
			fmt.Println(err)
		}

		for _, v := range dynamicValues {
			expr, err := govaluate.NewEvaluableExpression(v.Condition)
			if err != nil {
				fmt.Println(err)
			}

			parameters := make(gin.H)
			for _, va := range expr.Vars() {
				if HasPrefix(va, "json_") {
					parameters[va] = jj.GetBytes(reqBody, va[5:]).Value()
				}
			}

			evaluateResult, err := expr.Evaluate(parameters)
			if err != nil {
				fmt.Println(err)
			}

			if yes, ok := evaluateResult.(bool); ok && yes {
				fmt.Println(string(v.Response))
			}
		}
	}
}

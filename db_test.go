package httplive_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Knetic/govaluate"
	"github.com/bingoohuang/httplive"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

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

	result := gjson.GetBytes(j, "_dynamic")
	fmt.Println(result)

	reqBody := []byte(`
{
	"name":"bingoo"
}
`)

	if result.Type == gjson.JSON {
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
				if httplive.HasPrefix(va, "json_") {
					parameters[va] = gjson.GetBytes(reqBody, va[5:]).Value()
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

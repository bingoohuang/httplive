package eval

import (
	"encoding/json"
	"github.com/bingoohuang/jj"
)

type MergeJSONEvaluator struct{}

func init() { registerEvaluator("@merge-json", &MergeJSONEvaluator{}) }

func (d MergeJSONEvaluator) Eval(ctx *Context, key, param string) EvaluatorResult {
	jp := jj.Parse(param)
	objects := jp.Get("objects").Array()
	by := jp.Get("by").String()

	m := make([]map[string]interface{}, 0)
	for i := 0; i < len(objects); i++ {
		objj := jj.Parse(ctx.Var(objects[i].Str).(string))

		for j, bi := range objj.Array() {
			if len(m) <= j {
				m = append(m, make(map[string]interface{}))
			}

			byValue := bi.Get(by).String()
			m[j][by] = byValue
			bi.ForEach(func(key, value jj.Result) bool {
				if key.String() == by {
					return true
				}

				m[j][key.String()] = json.RawMessage(value.Raw)
				return true
			})
		}
	}

	mj, _ := json.Marshal(m)

	return EvaluatorResult{
		Mode: EvaluatorSetRaw,
		Key:  key,
		Val:  string(mj),
	}
}

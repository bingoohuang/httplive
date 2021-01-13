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
	fulfil := jp.Get("fulfil")

	byMap := make(map[string]int)
	m := make([]map[string]interface{}, 0)
	objj := jj.Parse(ctx.Var(objects[0].Str).(string))
	for j, bi := range objj.Array() {
		m = append(m, make(map[string]interface{}))

		byValue := bi.Get(by).String()
		m[j][by] = byValue
		byMap[byValue] = j

		bi.ForEach(func(key, value jj.Result) bool {
			m[j][key.String()] = json.RawMessage(value.Raw)

			return true
		})
	}

	for i := 1; i < len(objects); i++ {
		objj = jj.Parse(ctx.Var(objects[i].Str).(string))

		for _, bi := range objj.Array() {
			byValue := bi.Get(by).String()
			m0, ok := byMap[byValue]
			if !ok {
				m1 := make(map[string]interface{})
				m1[by] = byValue
				m0 = len(m)
				byMap[byValue] = m0
				m = append(m, m1)
			}

			bi.ForEach(func(key, value jj.Result) bool {
				m[m0][key.String()] = json.RawMessage(value.Raw)
				return true
			})
		}
	}

	if fulfil.IsObject() {
		for _, i := range m {
			fulfil.ForEach(func(key, value jj.Result) bool {
				if _, ok := i[key.String()]; !ok {
					i[key.String()] = value.Value()
				}
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

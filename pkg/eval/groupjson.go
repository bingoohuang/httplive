package eval

import (
	"encoding/json"
	"github.com/bingoohuang/jj"
)

type GroupJsonEvaluator struct{}

func init() { registerEvaluator("@group-json", &GroupJsonEvaluator{}) }

func (d GroupJsonEvaluator) Eval(ctx *Context, key, param string) EvaluatorResult {
	jp := jj.Parse(param)
	param = intervalEval(ctx, param, jp)
	jp = jj.Parse(param)

	groupBy := jp.Get("group-by").String()
	flattenKey := jp.Get("flatten-key").String()
	flattenValuesKey := jp.Get("flatten-values").String()
	fillStart := tryToFloat64(jp.Get("fill-start").String())
	fillEnd := tryToFloat64(jp.Get("fill-end").String())
	fillStep := tryToFloat64(jp.Get("fill-step").String()).(float64)
	fill := jp.Get("fill").String()
	fillValue := tryToFloat64(fill)

	srcJson := ctx.Var(key)
	srcJJ := jj.Parse(string(srcJson.(RawString)))

	lastGroupByValue := ""
	flattenArray := make([]interface{}, 0)
	flatten := make([]interface{}, 0)
	lastFlattenKeyValue := fillStart

	f := func() {
		if fl, ok := lastFlattenKeyValue.(float64); ok {
			tf := fillEnd.(float64)
			for fl += fillStep; fl < tf; fl += fillStep {
				flattenArray = append(flattenArray, fillValue)
			}
		}

		v := map[string]interface{}{}
		v[flattenKey] = flattenArray
		v[groupBy] = lastGroupByValue
		flatten = append(flatten, v)
	}

	srcJJ.ForEach(func(_, value jj.Result) bool {
		groupByValue := value.Get(groupBy).String()
		flattenValue := value.Get(flattenValuesKey).Value()
		flattenKeyValue := value.Get(flattenKey).Value()

		if lastGroupByValue == "" || groupByValue == lastGroupByValue {
			flattenArray = append(flattenArray, flattenValue)
		}

		if fl, ok := lastFlattenKeyValue.(float64); ok {
			tf := flattenKeyValue.(float64)
			for fl++; fl < tf; fl++ {
				flattenArray = append(flattenArray, fillValue)
			}
		}

		if lastGroupByValue == "" || groupByValue == lastGroupByValue {
			lastGroupByValue = groupByValue
			lastFlattenKeyValue = flattenKeyValue
			return true
		}

		f()
		lastFlattenKeyValue = fillStart

		lastGroupByValue = groupByValue
		flattenArray = make([]interface{}, 0)

		return true
	})

	if len(flattenArray) > 0 {
		f()
	}

	flattenJSON, _ := json.Marshal(flatten)
	ctx.SetVar(key, string(flattenJSON))

	return EvaluatorResult{
		Mode: EvaluatorDel,
		Key:  key,
	}
}

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

	var stepper Stepper
	fill := jp.Get("fill")
	if fill.String() != "" {
		stepper = ctx.Var(fill.String()).(Stepper)
	}

	srcJson := ctx.Var(key)
	srcJJ := jj.Parse(string(srcJson.(RawString)))

	lastGroupByValue := ""
	flattenArray := make([]interface{}, 0)
	flatten := make([]interface{}, 0)

	f := func() {
		if stepper != nil {
			_, _, ok := stepper.Step()
			for ok {
				flattenArray = append(flattenArray, stepper.Fill())
				_, _, ok = stepper.Step()
			}
			stepper.Reset()
		}

		v := map[string]interface{}{}
		v[flattenKey] = flattenArray
		v[groupBy] = lastGroupByValue
		flatten = append(flatten, v)
		flattenArray = make([]interface{}, 0)
	}

	srcJJ.ForEach(func(_, value jj.Result) bool {
		groupByValue := value.Get(groupBy).String()
		groupChanged := lastGroupByValue != "" && groupByValue != lastGroupByValue
		if groupChanged {
			f()
		}

		if stepper != nil {
			flattenKeyValue := value.Get(flattenKey).Value()
			tf := stepper.Parse(flattenKeyValue.(float64))
			_, stepF, ok := stepper.Step()
			for ok && stepF <= tf {
				flattenArray = append(flattenArray, stepper.Fill())
				_, stepF, ok = stepper.Step()
			}
		}

		lastGroupByValue = groupByValue
		flattenValue := value.Get(flattenValuesKey).Value()
		flattenArray = append(flattenArray, flattenValue)
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

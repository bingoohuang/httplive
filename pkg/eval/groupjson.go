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
		stepper = fill.Value().(Stepper)
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

		if stepper != nil {
			tf := flattenKeyValue.(float64)
			_, stepF, ok := stepper.Step()
			for ok && stepF < tf {
				flattenArray = append(flattenArray, stepper.Fill())
				_, stepF, ok = stepper.Step()
			}
		}

		if lastGroupByValue == "" || groupByValue == lastGroupByValue {
			lastGroupByValue = groupByValue
			return true
		}

		f()

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

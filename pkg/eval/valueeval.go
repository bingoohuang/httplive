package eval

import (
	"github.com/antonmedv/expr"
	"strings"
)

type ValEvalEvaluator struct{}

func init() { registerEvaluator("@val-eval", &ValEvalEvaluator{}) }

type RawString string

func (d ValEvalEvaluator) Eval(ctx *Context, key, param string) EvaluatorResult {
	if param == "" {
		param = key
	}

	vars := make(map[string]interface{})
	for k, v := range exprFns {
		vars[k] = v
	}
	for k, v := range ctx.Vars {
		vars[k] = v
	}

	result, err := expr.Eval(strings.TrimSpace(param), vars)
	if err != nil {
		return EvaluatorResult{Err: err}
	}

	ctx.SetVar(key, result)

	mode := EvaluatorSet
	if v, ok := result.(RawString); ok {
		mode = EvaluatorSetRaw
		result = string(v)
	}

	return EvaluatorResult{
		Mode: mode,
		Key:  key,
		Val:  result,
	}
}

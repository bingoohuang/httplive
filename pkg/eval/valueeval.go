package eval

import (
	"strings"

	"github.com/bingoohuang/govaluate"
)

type ValEvalEvaluator struct{}

func init() { registerEvaluator("@val-eval", &ValEvalEvaluator{}) }

type RawString string

func (d ValEvalEvaluator) Eval(ctx *Context, key, param string) EvaluatorResult {
	if param == "" {
		param = key
	}
	expr, err := govaluate.NewEvaluableExpressionWithFunctions(strings.TrimSpace(param), exprFns)
	if err != nil {
		return EvaluatorResult{Err: err}
	}

	result, err := expr.Eval(govaluate.MapParameters(ctx.Vars))
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

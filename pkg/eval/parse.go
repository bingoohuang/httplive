package eval

import (
	"strings"

	"github.com/bingoohuang/jj"
)

func parseEvaluator(ctx *Context, k, v jj.Result) EvaluatorFn {
	ks, vs := k.String(), v.String()
	for evaluatorKey, evaluator := range evaluatorRegistry {
		if strings.HasPrefix(vs, evaluatorKey) {
			return func() EvaluatorResult {
				return evaluator.Eval(ctx, ks, vs[len(evaluatorKey):])
			}
		}

		if strings.HasSuffix(ks, evaluatorKey) {
			key := ks[:len(ks)-len(evaluatorKey)]
			return func() EvaluatorResult {
				return evaluator.Eval(ctx, key, v.Raw)
			}
		}
	}

	return nil
}

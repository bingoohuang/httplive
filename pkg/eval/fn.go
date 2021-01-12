package eval

import (
	"context"
	"io"
	"strings"

	"github.com/bingoohuang/jj"
)

type Evaluator interface {
	Eval(ctx *Context, key, param string) EvaluatorResult
}

var evaluatorRegistry = make(map[string]Evaluator)

func registerEvaluator(name string, evaluator Evaluator) {
	evaluatorRegistry[name] = evaluator
}

type EvaluatorMode int

const (
	EvaluatorSet EvaluatorMode = iota
	EvaluatorSetRaw
	EvaluatorDel
)

type EvaluatorResult struct {
	Mode EvaluatorMode
	Key  string
	Val  interface{}
	Err  error
}

type EvaluatorFn func() EvaluatorResult

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

type Context struct {
	Vars map[string]interface{}
}

func (c *Context) SetVar(name string, value interface{}) {
	c.Vars[name] = value
}

func (c *Context) Var(varName string) interface{} {
	return c.Vars[varName]
}

func (c *Context) Close() {
	for _, v := range c.Vars {
		if vv, ok := v.(io.Closer); ok {
			vv.Close()
		}
	}
}

func NewContext() *Context {
	return &Context{
		Vars: make(map[string]interface{}),
	}
}

func JSONStr(gj jj.Result, k string) string {
	v := gj.Get(k)
	return v.String()
}

func JSONStrOr(gj jj.Result, k, defaultV string) string {
	v := gj.Get(k)
	s := v.String()
	if s == "" {
		return defaultV
	}

	return s
}

func JSONInt(gj jj.Result, k string) int {
	v := gj.Get(k)
	return int(v.Int())
}

var background = context.Background()

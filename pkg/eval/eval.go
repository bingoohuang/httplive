package eval

import (
	"github.com/bingoohuang/jj"
	"log"
	"strings"
)

func Eval(body string) string {
	_hl := jj.Get(body, "_hl")
	if !(_hl.Type == jj.String && _hl.String() == "eval") {
		return body
	}

	body, err := jj.Delete(body, "_hl")
	if err != nil {
		log.Printf("failed to delete %s in json, error:%v", "_hl", err)
		return body
	}

	root := jj.Parse(body)
	ctx := NewContext()
	defer ctx.Close()

	return intervalEval(body, root, ctx)
}

func intervalEval(body string, root jj.Result, ctx *Context) string {
	setOptions := jj.SetOptions{ReplaceInPlace: true}

	root.ForEach(func(k, v jj.Result) bool {
		kk := k.String()
		if strings.HasPrefix(kk, "#") || strings.HasPrefix(kk, "//") {
			body, _ = jj.Delete(body, kk)
			return true
		}

		if evaluator := parseEvaluator(ctx, k, v); evaluator != nil {
			body = doEval(evaluator, body, kk, setOptions)
			return true
		}

		if v.IsObject() {
			sub := intervalEval(v.Raw, v, ctx)
			body, _ = jj.SetRaw(body, kk, sub, setOptions)
		}

		return true
	})

	return body
}

func doEval(evaluator EvaluatorFn, body string, kk string, setOptions jj.SetOptions) string {
	evaluated := evaluator()
	err := evaluated.Err
	if err != nil {
		log.Printf("W! error: %v", err)
		return ""
	}

	switch evaluated.Mode {
	case EvaluatorSet:
		body, err = jj.Set(body, kk, evaluated.Val, setOptions)
	case EvaluatorSetRaw:
		body, err = jj.SetRaw(body, kk, evaluated.Val.(string), setOptions)
	case EvaluatorDel:
		body, err = jj.Delete(body, kk, setOptions)
	}

	if err != nil {
		log.Printf("failed to delete %s in json, error:%v", "_hl", err)
	}

	return body
}

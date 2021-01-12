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

	setOptions := jj.SetOptions{ReplaceInPlace: true}

	root.ForEach(func(k, v jj.Result) bool {
		kk := k.String()
		if strings.HasPrefix(kk, "#") || strings.HasPrefix(kk, "//") {
			body, err = jj.Delete(body, kk)
			return true
		}

		evaluator := parseEvaluator(ctx, k, v)
		if evaluator != nil {
			evaluated := evaluator()
			err := evaluated.Err
			if err != nil {
				log.Printf("W! error: %v", err)
				return true
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
		}

		return true
	})

	return body
}

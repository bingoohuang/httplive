package eval

import (
	"log"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func Eval(body string) string {
	_hl := gjson.Get(body, "_hl")
	if !(_hl.Type == gjson.String && _hl.String() == "eval") {
		return body
	}

	body, err := sjson.Delete(body, "_hl")
	if err != nil {
		log.Printf("failed to delete %s in json, error:%v", "_hl", err)
		return body
	}

	root := gjson.Parse(body)
	ctx := NewContext()
	defer ctx.Close()

	root.ForEach(func(k, v gjson.Result) bool {
		kk := k.String()
		if strings.HasPrefix(kk, "#") || strings.HasPrefix(kk, "//") {
			body, err = sjson.Delete(body, kk)
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
				body, err = sjson.Set(body, kk, evaluated.Val)
			case EvaluatorSetRaw:
				body, err = sjson.SetRaw(body, kk, evaluated.Val.(string))
			case EvaluatorDel:
				body, err = sjson.Delete(body, kk)
			}

			if err != nil {
				log.Printf("failed to delete %s in json, error:%v", "_hl", err)
			}
		}

		return true
	})

	return body
}

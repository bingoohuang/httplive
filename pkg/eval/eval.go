package eval

import (
	"github.com/patrickmn/go-cache"
	"log"
	"strings"
	"time"

	"github.com/bingoohuang/jj"
)

// Create a cache with a default expiration time of 5 minutes, and which
// purges expired items every 10 minutes
var evalCache = cache.New(5*time.Minute, 10*time.Minute)

func Eval(endpoint string, body string) string {
	_hl := jj.Get(body, "_hl")
	if !(_hl.Type == jj.String && _hl.String() == "eval") {
		return body
	}

	body, err := jj.Delete(body, "_hl")
	if err != nil {
		log.Printf("W! failed to delete %s in json, error:%v", "_hl", err)
		return body
	}

	cacheTime := time.Duration(0)
	_cache := jj.Get(body, "_cache")
	if _cache.Type == jj.String {
		cacheTime, err = time.ParseDuration(_cache.String())
		if err != nil {
			log.Printf("W! failed to parse cache time:%s, error:%v", _cache.String(), err)
		}
	}

	if cacheTime > 0 {
		if result, ok := evalCache.Get(endpoint); ok {
			return result.(string)
		}
	}

	root := jj.Parse(body)
	ctx := NewContext()
	defer ctx.Close()

	evalResult := intervalEval(body, root, ctx)
	if cacheTime > 0 {
		// Set the value of the key "foo" to "bar", with the default expiration time
		evalCache.Set(endpoint, evalResult, cacheTime)
	}

	return evalResult
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
		return body
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

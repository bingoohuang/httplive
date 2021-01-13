package eval

import (
	"log"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"

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

	f := func() string {
		root := jj.Parse(body)
		ctx := NewContext()
		defer ctx.Close()

		evalResult := intervalEval(ctx, body, root)
		if cacheTime > 0 {
			// Set the value of the key "foo" to "bar", with the default expiration time
			evalCache.Set(endpoint, evalResult, cacheTime)
		}

		return evalResult
	}

	evalResult := ""

	if cacheTime > 0 {
		result, expired, ok := evalCache.GetWithExpiration(endpoint)
		if ok {
			evalResult = result.(string)
			if time.Until(expired) <= 10*time.Second {
				go f()
			}

			return evalResult
		}
	}

	return f()
}

func intervalEval(ctx *Context, body string, root jj.Result) string {
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
			sub := intervalEval(ctx, v.Raw, v)
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
		body, err = jj.Set(body, evaluated.Key, evaluated.Val, setOptions)
	case EvaluatorSetRaw:
		body, err = jj.SetRaw(body, evaluated.Key, evaluated.Val.(string), setOptions)
	case EvaluatorDel:
		body, err = jj.Delete(body, kk, setOptions)
	}

	if err != nil {
		log.Printf("failed to delete %s in json, error:%v", "_hl", err)
	}

	return body
}

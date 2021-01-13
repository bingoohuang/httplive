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

	f0 := func() string {
		ctx := NewContext()
		defer ctx.Close()

		return string(jj.Ugly([]byte(intervalEval(ctx, body, jj.Parse(body)))))
	}

	cacheTime := parseCacheTime(body, err)
	if cacheTime <= 0 {
		return f0()
	}

	f1 := func() string {
		s := f0()
		evalCache.Set(endpoint, s, cacheTime+10*time.Second)
		return s
	}

	if r, exp, ok := evalCache.GetWithExpiration(endpoint); ok {
		if time.Until(exp) <= 10*time.Second {
			go f1()
		}

		return r.(string)
	}

	return f1()
}

func parseCacheTime(body string, err error) time.Duration {
	cacheTime := time.Duration(0)
	if v := jj.Get(body, "_cache"); v.Type == jj.String {
		if cacheTime, err = time.ParseDuration(v.String()); err != nil {
			log.Printf("W! failed to parse cache time:%s, error:%v", v.String(), err)
		}
	}

	return cacheTime
}

func intervalEval(ctx *Context, body string, root jj.Result) string {
	setOptions := jj.SetOptions{ReplaceInPlace: true}

	root.ForEach(func(k, v jj.Result) bool {
		kk := k.String()
		if strings.HasPrefix(kk, "#") || strings.HasPrefix(kk, "//") {
			body, _ = jj.Delete(body, kk)
		} else if evaluator := parseEvaluator(ctx, k, v); evaluator != nil {
			body = doEval(evaluator, body, kk, setOptions)
		} else if v.IsObject() {
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

package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/tidwall/gjson"
)

type Replacer struct {
	Items []ReplaceItem
}

type ReplaceItem struct {
	Reg *regexp.Regexp
	Val string
}

func ParseReplacer(expr string) (*Replacer, error) {
	items := make([]ReplaceItem, 0)
	for _, item := range strings.Fields(expr) {
		subs := strings.SplitN(item, "=>", 2)
		k, v := subs[0], subs[1]
		var kreg *regexp.Regexp
		if strings.HasSuffix(k, ",i") {
			kreg = regexp.MustCompile("(?i)" + k[:len(k)-2])
		} else {
			kreg = regexp.MustCompile(k)
		}

		items = append(items, ReplaceItem{Reg: kreg, Val: v})
	}

	return &Replacer{Items: items}, nil
}

func (r *Replacer) Replace(str string) string {
	s := str
	for _, item := range r.Items {
		s = item.Reg.ReplaceAllString(s, item.Val)
	}
	return s
}

var dateReplacer, _ = ParseReplacer("yyyy,i=>2006 MM=>01 dd,i=>02 HH=>15 hh=>03 mm=>04 sss,i=>000 ss,i=>05")

type Evaluator interface {
	Eval(ctx *Context, key, param string) EvaluatorResult
}

var evaluatorRegistry = make(map[string]Evaluator)

func registerEvaluator(name string, evaluator Evaluator) {
	evaluatorRegistry[name] = evaluator
}

type DateEvaluator struct{}

func init() { registerEvaluator("@date", &DateEvaluator{}) }

func (d DateEvaluator) Eval(ctx *Context, key, param string) EvaluatorResult {
	dateFmt := "yyyy-MM-dd hh:mm:ss.SSS"
	if len(param) > 0 {
		dateFmt = param[1:]
	}
	dateFmt = dateReplacer.Replace(dateFmt)

	return EvaluatorResult{
		Mode: EvaluatorSet,
		Key:  key,
		Val:  time.Now().Format(dateFmt),
	}
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
	Val  string
	Err  error
}

type EvaluatorFn func() EvaluatorResult

func parseEvaluator(ctx *Context, k, v gjson.Result) EvaluatorFn {
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

func NewContext() *Context {
	return &Context{
		Vars: make(map[string]interface{}),
	}
}

type RedisVarEvaluator struct{}

func (r RedisVarEvaluator) Eval(ctx *Context, key, param string) EvaluatorResult {
	jparam := gjson.Parse(param)
	opt := &redis.Options{
		Addr:     JSONStr(jparam, "addr"),
		Password: JSONStr(jparam, "password"),
		DB:       JSONInt(jparam, "db"),
	}
	rdb := redis.NewClient(opt)
	ctx.SetVar(key, rdb)

	return EvaluatorResult{
		Mode: EvaluatorDel,
		Key:  key,
	}
}

func JSONStr(gj gjson.Result, k string) string {
	v := gj.Get(k)
	return v.String()
}

func JSONInt(gj gjson.Result, k string) int {
	v := gj.Get(k)
	return int(v.Int())
}

func init() { registerEvaluator("@redis-instance", &RedisVarEvaluator{}) }

type RedisGetEvaluator struct{}

var background = context.Background()

func (r RedisGetEvaluator) Eval(ctx *Context, key, param string) EvaluatorResult {
	params := strings.Fields(param)
	redisInstance := params[0]
	redisClient, _ := ctx.Var(redisInstance).(*redis.Client)
	if redisClient == nil {
		return EvaluatorResult{
			Err: fmt.Errorf("unable to find redis instance %s", redisInstance),
		}
	}

	keys := params[1:]
	value := ""
	if len(keys) == 1 {
		value, _ = redisClient.Get(background, keys[0]).Result()
	} else if len(keys) > 1 {
		value, _ = redisClient.HGet(background, keys[0], keys[1]).Result()
	}

	mode := EvaluatorSet
	if json.Valid([]byte(value)) {
		mode = EvaluatorSetRaw
	}

	return EvaluatorResult{
		Mode: mode,
		Key:  key,
		Val:  value,
	}
}

func init() { registerEvaluator("@redis", &RedisGetEvaluator{}) }

package eval

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bingoohuang/jj"
	"github.com/go-redis/redis/v8"
)

type RedisInstance struct{}

func init() { registerEvaluator("@redis-instance", &RedisInstance{}) }

func (r RedisInstance) Eval(ctx *Context, key, param string) EvaluatorResult {
	jparam := jj.Parse(param)
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

type RedisGetEvaluator struct{}

func init() { registerEvaluator("@redis", &RedisGetEvaluator{}) }

func (r RedisGetEvaluator) Eval(ctx *Context, key, param string) EvaluatorResult {
	params := strings.Fields(param)
	instance := params[0]
	redisClient, _ := ctx.Var(instance).(*redis.Client)
	if redisClient == nil {
		return EvaluatorResult{
			Err: fmt.Errorf("unable to find redis instance %s", instance),
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

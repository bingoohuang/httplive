package eval

import (
	"log"
	"time"

	"github.com/bingoohuang/jj"
)

type DateEvaluator struct{}

func init() { registerEvaluator("@date", &DateEvaluator{}) }

var dateReplacer, _ = ParseReplacer("yyyy,i=>2006 MM=>01 dd,i=>02 HH=>15 hh=>03 mm=>04 sss,i=>000 ss,i=>05")

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

type TimeEvaluator struct{}

func init() { registerEvaluator("@time", &TimeEvaluator{}) }

func (d TimeEvaluator) Eval(ctx *Context, key, param string) EvaluatorResult {
	p := jj.Parse(param)
	value := p.Get("value").String()
	unit := p.Get("unit").String()
	truncate := p.Get("truncate").String()

	result := EvaluatorResult{
		Mode: EvaluatorDel,
		Key:  key,
	}

	switch value {
	case "today":
		ctx.SetVar(key, todayTime(time.Now(), unit, truncate))
	case "tomorrow":
		ctx.SetVar(key, todayTime(time.Now().Add(24*time.Hour), unit, truncate))
	}

	return result
}

func todayTime(t time.Time, unit string, truncate string) interface{} {
	d := time.Duration(0)
	if truncate != "" {
		v, err := time.ParseDuration(truncate)
		if err != nil {
			log.Printf("W! failed to parse truncate %s, err: %v", truncate, err)
		} else {
			d = v
		}

	}
	switch unit {
	case "s", "seconds":
		return t.Truncate(d).Unix()
	default:
		if unit == "" {
			unit = "2006-01-02 15:04:05"
		}

		return t.Format(dateReplacer.Replace(unit))
	}
}

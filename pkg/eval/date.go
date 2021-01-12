package eval

import (
	"github.com/bingoohuang/golog/pkg/timex"
	"log"
	"time"

	"github.com/bingoohuang/jj"
)

type DateEvaluator struct{}

func init() { registerEvaluator("@date", &DateEvaluator{}) }

var dateReplacer, _ = ParseReplacer("yyyy,i=>2006 MM=>01 dd,i=>02 HH=>15 hh=>03 mm=>04 sss,i=>000 ss,i=>05")

func (d DateEvaluator) Eval(ctx *Context, key, param string) EvaluatorResult {
	jp := jj.Parse(param)
	dateFmt := "yyyy-MM-dd hh:mm:ss.SSS"
	offset := time.Duration(0)

	if jp.IsObject() {
		evalParam := intervalEval(ctx, jp.Raw, jp)
		jp = jj.Parse(evalParam)
		dateFmt = JSONStrOr(jp, "fmt", dateFmt)
		offset, _ = timex.ParseDuration(JSONStrOr(jp, "offset", "0"))
	} else {
		if len(param) > 0 {
			dateFmt = param[1:]
		}
	}
	dateFmt = dateReplacer.Replace(dateFmt)

	return EvaluatorResult{
		Mode: EvaluatorSet,
		Key:  key,
		Val:  time.Now().Add(offset).Format(dateFmt),
	}
}

type TimeEvaluator struct{}

func init() { registerEvaluator("@time", &TimeEvaluator{}) }

func (d TimeEvaluator) Eval(ctx *Context, key, param string) EvaluatorResult {
	p := jj.Parse(param)
	value := p.Get("value").String()
	unit := p.Get("unit").String()
	truncate := p.Get("truncate").String()
	offset := p.Get("offset").String()

	result := EvaluatorResult{
		Mode: EvaluatorDel,
		Key:  key,
	}

	switch value {
	case "today":
		ctx.SetVar(key, timeValue(time.Now(), offset, unit, truncate))
	case "tomorrow":
		ctx.SetVar(key, timeValue(time.Now().Add(24*time.Hour), offset, unit, truncate))
	}

	return result
}

func parseDuration(duration, name string) time.Duration {
	if duration == "" {
		return 0
	}

	v, err := timex.ParseDuration(duration)
	if err != nil {
		log.Printf("W! failed to parse %s %s, err: %v", name, duration, err)
	}
	return v
}

func timeValue(t time.Time, offset, unit, truncate string) interface{} {
	d := parseDuration(truncate, "truncate")
	off := parseDuration(offset, "offset")
	if off != 0 {
		t = t.Add(off)
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

type DateStepEvaluator struct{}

func init() { registerEvaluator("@date-step", &DateStepEvaluator{}) }

func (d DateStepEvaluator) Eval(ctx *Context, key, param string) EvaluatorResult {
	jp := jj.Parse(param)
	stepV := jp.Get("step").String()
	step, _ := timex.ParseDuration(stepV)
	dateFmt := JSONStr(jp, "fmt")
	dateFmt = dateReplacer.Replace(dateFmt)

	return EvaluatorResult{
		Mode: EvaluatorSet,
		Key:  key,
		Val:  time.Time{}.Add(step).Format(dateFmt),
	}
}

package eval

import (
	"log"
	"time"

	"github.com/bingoohuang/golog/pkg/timex"
	"github.com/bingoohuang/httplive/pkg/timx"
	"github.com/bingoohuang/jj"
)

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
	case "monthStart":
		ctx.SetVar(key, timeValue(timx.Time(time.Now()).BeginningOfMonth(), offset, unit, truncate))
	case "nextMonthStart":
		ctx.SetVar(key, timeValue(timx.Time(time.Now()).BeginningOfNextMonth(), offset, unit, truncate))
	case "monthEnd":
		ctx.SetVar(key, timeValue(timx.Time(time.Now()).EndOfMonth(), offset, unit, truncate))
	case "dayStart":
		ctx.SetVar(key, timeValue(timx.Time(time.Now()).BeginningOfDay(), offset, unit, truncate))
	case "nextDayStart":
		ctx.SetVar(key, timeValue(timx.Time(time.Now()).BeginningOfNextDay(), offset, unit, truncate))
	case "dayEnd":
		ctx.SetVar(key, timeValue(timx.Time(time.Now()).EndOfDay(), offset, unit, truncate))
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
		return timx.Time(t).TruncateTime(d).Unix()
	default:
		if unit == "" {
			unit = "2006-01-02 15:04:05"
		}

		return t.Format(dateReplacer.Replace(unit))
	}
}

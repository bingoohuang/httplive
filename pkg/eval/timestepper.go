package eval

import (
	"fmt"
	"time"

	"github.com/bingoohuang/golog/pkg/timex"
	"github.com/bingoohuang/httplive/pkg/timx"
	"github.com/bingoohuang/jj"
)

type TimeStepperEvaluator struct{}

func init() { registerEvaluator("@time-stepper", &TimeStepperEvaluator{}) }

type Stepper interface {
	Fill() interface{}
	Step() (string, float64, bool)
	Parse(f float64) float64
	Reset()
}

type TimeStepper struct {
	start, end time.Time
	current    time.Time
	fmt        string
	step       time.Duration
	fill       interface{}
}

func (s *TimeStepper) Reset() {
	s.current = s.start
}

func (s *TimeStepper) Parse(f float64) float64 {
	v, _ := time.ParseInLocation(s.fmt, fmt.Sprintf("%d", int64(f)), time.Local)
	return float64(v.UnixNano())
}

func (s *TimeStepper) Fill() interface{} {
	return s.fill
}

func (s *TimeStepper) Step() (string, float64, bool) {
	s.current = s.current.Add(s.step)
	ok := !s.current.After(s.end)
	return s.current.Format(s.fmt), float64(s.current.UnixNano()), ok
}

func (d TimeStepperEvaluator) Eval(ctx *Context, key, param string) EvaluatorResult {
	jp := jj.Parse(param)
	stepConf := jp.Get("step").String()
	step, _ := timex.ParseDuration(stepConf)
	startOffset, _ := timex.ParseDuration(jp.Get("startOffset").String())
	endOffset, _ := timex.ParseDuration(jp.Get("endOffset").String())
	dateFmt := JSONStr(jp, "fmt")
	dateFmt = dateReplacer.Replace(dateFmt)
	now := time.Now()

	var stepper Stepper = &TimeStepper{
		start: timx.Time(now.Add(startOffset)).TruncateTime(step),
		end:   timx.Time(now.Add(endOffset)).TruncateTime(step),
		fmt:   dateFmt,
		step:  step,
		fill:  jp.Get("fill").Value(),
	}

	stepper.Reset()

	ctx.SetVar(key, stepper)
	return EvaluatorResult{
		Mode: EvaluatorDel,
		Key:  key,
	}
}

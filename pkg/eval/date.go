package eval

import "time"

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

package eval

import (
	"time"

	"github.com/bingoohuang/golog/pkg/timex"
	"github.com/bingoohuang/httplive/pkg/util"
	"github.com/bingoohuang/jj"
)

type NowEvaluator struct{}

func init() { registerEvaluator("@now", &NowEvaluator{}) }

var dateReplacer, _ = util.ParseReplacer("yyyy,i=>2006 MM=>01 dd,i=>02 HH=>15 hh=>03 mm=>04 sss,i=>000 ss,i=>05")

func (d NowEvaluator) Eval(ctx *Context, key, param string) EvaluatorResult {
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

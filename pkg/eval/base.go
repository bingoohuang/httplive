package eval

type Evaluator interface {
	Eval(ctx *Context, key, param string) EvaluatorResult
}

var evaluatorRegistry = make(map[string]Evaluator)

func registerEvaluator(name string, evaluator Evaluator) {
	evaluatorRegistry[name] = evaluator
}

type EvaluatorMode int

const (
	EvaluatorSet EvaluatorMode = iota
	EvaluatorSetRaw
	EvaluatorDel
)

type EvaluatorResult struct {
	Val  interface{}
	Err  error
	Key  string
	Mode EvaluatorMode
}

type EvaluatorFn func() EvaluatorResult

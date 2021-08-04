package exprx

import (
	"errors"
	"fmt"
	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/ast"
	"github.com/antonmedv/expr/parser"
	"github.com/antonmedv/expr/vm"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
	"time"
)

type H map[string]interface{}

func TestEvaluate(t *testing.T) {
	out, err := expr.Eval("foo + bar", H{"foo": 1, "bar": 2})
	assert.Nil(t, err)
	assert.Equal(t, 3, out)

	out, err = expr.Eval("foo.bar", H{"foo": H{"bar": 2}})
	assert.Nil(t, err)
	assert.Equal(t, 2, out)

	out, err = expr.Eval("f.b", H{"f": H{"b": "2"}})
	assert.Nil(t, err)
	assert.Equal(t, "2", out)
}

func TestCompile(t *testing.T) {
	// Compile code into bytecode. This step can be done once and program may be reused.
	// Specify environment for type check.
	program, err := expr.Compile(`sprintf(greet, names[0])`)
	assert.Nil(t, err)

	out, err := expr.Run(program, H{
		"greet":   "Hello, %v!",
		"names":   []string{"world", "you"},
		"sprintf": fmt.Sprintf, // You can pass any functions.
	})
	assert.Nil(t, err)
	assert.Equal(t, "Hello, world!", out)
}

type Env struct {
	Tweets []Tweet
}

// Methods defined on such struct will be functions.
func (Env) Format(t time.Time) string { return t.Format(`2006-01-02 15:04:05`) }

type Tweet struct {
	Text string
	Date time.Time
}

var now, _ = time.ParseInLocation(`2006-01-02 15:04:05`, `2021-08-04 14:51:48`, time.Local)

func TestExistingTypes(t *testing.T) {
	code := `map(filter(Tweets, {len(.Text) > 0}), {.Text + ' '+ Format(.Date)})`
	// We can use an empty instance of the struct as an environment.
	program, err := expr.Compile(code, expr.Env(Env{}))
	assert.Nil(t, err)

	env := Env{Tweets: []Tweet{
		{Text: "Oh My God!", Date: now},
		{Text: "How you doing?", Date: now},
		{Text: "Could I be wearing any more clothes?", Date: now},
	}}
	output, err := expr.Run(program, env)
	assert.Nil(t, err)
	assert.Equal(t, []interface{}{
		`Oh My God! 2021-08-04 14:51:48`,
		`How you doing? 2021-08-04 14:51:48`,
		`Could I be wearing any more clothes? 2021-08-04 14:51:48`,
	}, output)
}

func TestCustomFunction(t *testing.T) {
	env := H{
		"foo":    1,
		"double": func(i int) int { return i * 2 },
	}

	out, err := expr.Eval("double(foo)", env)
	assert.Nil(t, err)
	assert.Equal(t, 2, out)
}

type FastEnv map[string]interface{}

func (FastEnv) FastMethod(...interface{}) interface{} {
	return "Hello, "
}

func TestFastMethod(t *testing.T) {
	env := FastEnv{
		"fast_func": func(...interface{}) interface{} { return "world" },
	}

	out, err := expr.Eval("FastMethod() + fast_func()", env)
	assert.Nil(t, err)
	assert.Equal(t, "Hello, world", out)
}

var ErrLessThanZero = errors.New("value cannot be less than zero")

func TestReturningError(t *testing.T) {
	env := H{
		"foo": -1,
		"double": func(i int) (int, error) {
			if i < 0 {
				return 0, ErrLessThanZero
			}
			return i * 2, nil
		},
	}

	out, err := expr.Eval("double(foo)", env)
	assert.True(t, errors.Is(err, ErrLessThanZero))
	assert.Nil(t, out)
}

func TestOperator(t *testing.T) {
	// We can define options before compiling.
	options := []expr.Option{
		expr.Env(EnvOperator{}),
		expr.Operator("-", "Sub"), // Override `-` with function `Sub`.
	}
	program, err := expr.Compile(`(Now() - CreatedAt).Hours() / 24 / 365`, options...)
	assert.Nil(t, err)

	env := EnvOperator{CreatedAt: time.Date(1987, time.November, 24, 20, 0, 0, 0, time.UTC)}
	output, err := expr.Run(program, env)
	assert.Nil(t, err)
	assert.Equal(t, 33.71630859969559, output)
}

type EnvOperator struct {
	datetime
	CreatedAt time.Time
}

// Functions may be defined on embedded structs as well.
type datetime struct{}

func (datetime) Now() time.Time                   { return now }
func (datetime) Sub(a, b time.Time) time.Duration { return a.Sub(b) }

func TestVisitor(t *testing.T) {
	tree, err := parser.Parse("foo + bar")
	assert.Nil(t, err)

	visitor := &visitor{}
	ast.Walk(&tree.Node, visitor)

	assert.Equal(t, []string{"foo", "bar"}, visitor.identifiers) // outputs [foo bar]
}

type visitor struct {
	identifiers []string
}

func (v *visitor) Enter(_ *ast.Node) {}
func (v *visitor) Exit(node *ast.Node) {
	if n, ok := (*node).(*ast.IdentifierNode); ok {
		v.identifiers = append(v.identifiers, n.Value)
	}
}

func TestPatch(t *testing.T) {
	env := map[string]interface{}{"list": []int{1, 2, 3}}
	code := `list[-1]` // will output 3

	program, err := expr.Compile(code, expr.Env(env), expr.Patch(&patcher{}))
	assert.Nil(t, err)

	output, err := expr.Run(program, env)
	assert.Nil(t, err)
	assert.Equal(t, 3, output)
}

type patcher struct{}

func (p *patcher) Enter(_ *ast.Node) {}
func (p *patcher) Exit(node *ast.Node) {
	n, ok := (*node).(*ast.IndexNode)
	if !ok {
		return
	}
	if unary, ok := n.Index.(*ast.UnaryNode); ok && unary.Operator == "-" {
		ast.Patch(&n.Index, &ast.BinaryNode{
			Operator: "-",
			Left:     &ast.BuiltinNode{Name: "len", Arguments: []ast.Node{n.Node}},
			Right:    unary.Node,
		})
	}
}

func TestStringer(t *testing.T) {
	code := `Price == "$100"`
	program, err := expr.Compile(code, expr.Env(EnvStringer{}), expr.Patch(&stringerPatcher{}))
	assert.Nil(t, err)

	output, err := expr.Run(program, EnvStringer{Price: 100_00})
	assert.Nil(t, err)
	assert.True(t, true, output)
}

type EnvStringer struct {
	Price Price
}

type Price int

func (p Price) String() string { return fmt.Sprintf("$%v", int(p)/100) }

var stringer = reflect.TypeOf((*fmt.Stringer)(nil)).Elem()

type stringerPatcher struct{}

func (p *stringerPatcher) Enter(_ *ast.Node) {}
func (p *stringerPatcher) Exit(node *ast.Node) {
	if t := (*node).Type(); t != nil && t.Implements(stringer) {
		ast.Patch(node, &ast.MethodNode{Node: *node, Method: "String"})
	}
}

func TestReuseVM(t *testing.T) {
	program, err := expr.Compile("foo + bar")
	assert.Nil(t, err)

	// Reuse this vm instance between runs
	v := vm.VM{}

	out, err := v.Run(program, H{"foo": 1, "bar": 2})
	assert.Nil(t, err)
	assert.Equal(t, 3, out)

	out, err = v.Run(program, H{"foo": 10, "bar": 20})
	assert.Nil(t, err)
	assert.Equal(t, 30, out)
}

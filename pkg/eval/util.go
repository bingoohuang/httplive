package eval

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bingoohuang/jj"
)

func JSONStr(gj jj.Result, k string) string {
	return gj.Get(k).String()
}

func JSONStrSep(gj jj.Result, k, arraySep string) string {
	v := gj.Get(k)
	if v.IsArray() {
		s := ""
		for _, item := range v.Array() {
			s += item.String() + arraySep
		}

		return s
	}

	return v.String()
}

func JSONStrOr(gj jj.Result, k, defaultV string) string {
	v := gj.Get(k)
	s := v.String()
	if s == "" {
		return defaultV
	}

	return s
}

func JSONInt(gj jj.Result, k string) int {
	v := gj.Get(k)
	return int(v.Int())
}

var background = context.Background()

func tryToFloat64(s string) interface{} {
	if s == "" {
		return s
	}

	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}

	return s
}

func purifyColumnName(col string) string {
	p := strings.LastIndexByte(col, '(')
	if p < 0 {
		return col
	}

	xx := col[p+1:]
	if p = strings.IndexByte(xx, ')'); p < 0 {
		return col
	}

	return xx[:p]
}

type ExpressionFunction func(arguments ...interface{}) (interface{}, error)

var exprFns = map[string]ExpressionFunction{
	"toInt": func(args ...interface{}) (interface{}, error) {
		arg0 := args[0]
		switch v := arg0.(type) {
		case int8:
			return float64(v), nil
		case int16:
			return float64(v), nil
		case int32:
			return float64(v), nil
		case int64:
			return float64(v), nil
		case uint8:
			return float64(v), nil
		case uint16:
			return float64(v), nil
		case uint32:
			return float64(v), nil
		case uint64:
			return float64(v), nil
		case float32:
			return float64(v), nil
		case float64:
			return v, nil
		}
		vs := fmt.Sprintf("%s", arg0)
		return strconv.ParseFloat(vs, 64)
	},
}

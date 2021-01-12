package eval

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/bingoohuang/jj"
	"log"
	"strconv"
	"strings"

	"github.com/bingoohuang/govaluate"
	_ "github.com/go-sql-driver/mysql"
)

type DbInstance struct{}

func init() { registerEvaluator("@db-instance", &DbInstance{}) }

func (d DbInstance) Eval(ctx *Context, key, param string) EvaluatorResult {
	jparam := jj.Parse(param)
	// db_user:db_pwd@tcp(localhost:3306)/my_db?charset=utf8mb4&parseTime=true&loc=Local&timeout=10s&writeTimeout=10s&readTimeout=10s
	driverName := JSONStrOr(jparam, "driver", "mysql")
	dsn := JSONStr(jparam, "dsn")
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return EvaluatorResult{Err: err}
	}

	ctx.SetVar(key, db)

	return EvaluatorResult{
		Mode: EvaluatorDel,
		Key:  key,
	}
}

type DbQueryEvaluator struct{}

func init() { registerEvaluator("@db-query", &DbQueryEvaluator{}) }

func (d DbQueryEvaluator) Eval(ctx *Context, key, param string) EvaluatorResult {
	jparam := jj.Parse(param)
	instance := JSONStr(jparam, "instance")
	resultType := JSONStr(jparam, "resultType")

	db, _ := ctx.Var(instance).(*sql.DB)
	if db == nil {
		return EvaluatorResult{
			Err: fmt.Errorf("unable to find DB instance %s", instance),
		}
	}

	query := JSONStrSep(jparam, "query", " ")
	log.Printf("I! query: %s", query)
	rows, err := db.Query(query)
	if err != nil {
		return EvaluatorResult{Err: err}
	}

	defer rows.Close()
	columns, _ := rows.Columns()
	columnSize := len(columns)

	for i, col := range columns {
		columns[i] = purifyColumnName(col)
	}

	rowsData := make([]map[string]string, 0)

	for rows.Next() {
		holders := make([]sql.NullString, columnSize)
		pointers := make([]interface{}, columnSize)
		for i := 0; i < columnSize; i++ {
			pointers[i] = &holders[i]
		}

		if err := rows.Scan(pointers...); err != nil {
			return EvaluatorResult{Err: err}
		}

		values := make(map[string]string)
		for i, h := range holders {
			values[columns[i]] = h.String
		}

		if resultType == "map" || resultType == "" {
			ctx.SetVar(key, values)
			break
		}

		rowsData = append(rowsData, values)
	}

	if resultType == "json" {
		rowsJSON, _ := json.Marshal(rowsData)
		ctx.SetVar(key, RawString(rowsJSON))
	}

	return EvaluatorResult{
		Mode: EvaluatorDel,
		Key:  key,
	}
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

type DbValueEvaluator struct{}

func init() { registerEvaluator("@db-value", &DbValueEvaluator{}) }

type RawString string

func (d DbValueEvaluator) Eval(ctx *Context, key, param string) EvaluatorResult {
	expr, err := govaluate.NewEvaluableExpressionWithFunctions(strings.TrimSpace(param), exprFns)
	if err != nil {
		return EvaluatorResult{Err: err}
	}

	result, err := expr.Eval(govaluate.MapParameters(ctx.Vars))
	if err != nil {
		return EvaluatorResult{Err: err}
	}

	mode := EvaluatorSet
	if v, ok := result.(RawString); ok {
		mode = EvaluatorSetRaw
		result = string(v)
	}

	return EvaluatorResult{
		Mode: mode,
		Key:  key,
		Val:  result,
	}
}

var exprFns = map[string]govaluate.ExpressionFunction{
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
			return float64(v), nil
		}
		vs := fmt.Sprintf("%s", arg0)
		return strconv.ParseFloat(vs, 64)
	},
}

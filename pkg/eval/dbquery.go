package eval

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/bingoohuang/httplive/pkg/placeholder"
	"github.com/bingoohuang/jj"
)

type DbQueryEvaluator struct{}

func init() { registerEvaluator("@db-query", &DbQueryEvaluator{}) }

func (d DbQueryEvaluator) Eval(ctx *Context, key, param string) EvaluatorResult {
	jparam := jj.Parse(param)
	instance := JSONStrOr(jparam, "instance", "default")
	resultType := JSONStr(jparam, "resultType")
	maxRows := JSONInt(jparam, "maxRows")

	dbInstance := ctx.Var(instance)
	db, _ := dbInstance.(*DbReuse)
	if db == nil {
		return EvaluatorResult{
			Err: fmt.Errorf("unable to find DB instance %s", instance),
		}
	}

	query := JSONStrSep(jparam, "query", " ")

	pl, err := placeholder.Create(query, ctx.Vars, "?")
	if err != nil {
		return EvaluatorResult{Err: err}
	}

	log.Printf("I! query: %s with vars:%v", pl.Value, pl.Vars)

	rows, err := db.Query(pl.Value, pl.Vars...)
	if err != nil {
		return EvaluatorResult{Err: err}
	}

	defer rows.Close()
	columns, _ := rows.Columns()
	columnSize := len(columns)

	for i, col := range columns {
		columns[i] = purifyColumnName(col)
	}

	rowsData := make([]map[string]interface{}, 0)

	for i := 0; rows.Next() && (maxRows == 0 || i < maxRows); i++ {
		holders := make([]sql.NullString, columnSize)
		pointers := make([]interface{}, columnSize)
		for i := 0; i < columnSize; i++ {
			pointers[i] = &holders[i]
		}

		if err := rows.Scan(pointers...); err != nil {
			return EvaluatorResult{Err: err}
		}

		values := make(map[string]interface{})
		for i, h := range holders {
			values[columns[i]] = tryToFloat64(h.String)
		}

		if resultType == "map" || resultType == "" || resultType == "json-object" {
			if resultType == "map" || resultType == "" {
				ctx.SetVar(key, values)
			} else {
				valuesJSON, _ := json.Marshal(values)
				ctx.SetVar(key, RawString(valuesJSON))
			}

			break
		}

		rowsData = append(rowsData, values)
	}

	if resultType == "json-array" {
		rowsJSON, _ := json.Marshal(rowsData)
		ctx.SetVar(key, RawString(rowsJSON))
	}

	return EvaluatorResult{
		Mode: EvaluatorDel,
		Key:  key,
	}
}

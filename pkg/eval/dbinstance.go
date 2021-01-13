package eval

import (
	"database/sql"

	"github.com/bingoohuang/jj"
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

package eval

import (
	"database/sql"
	"sync"

	"github.com/bingoohuang/jj"
)

type DbInstance struct{}

func init() { registerEvaluator("@db-instance", &DbInstance{}) }

var (
	globalDbMap  = make(map[string]*DbReuse)
	globalDbLock sync.Mutex
)

func (d DbInstance) Eval(ctx *Context, key, param string) EvaluatorResult {
	jparam := jj.Parse(param)
	global := jparam.Get("global").Bool()
	if global {
		if db, ok := GetDBReuse(key); ok {
			ctx.SetVar(key, db)
			return EvaluatorResult{Mode: EvaluatorDel, Key: key}
		}
	}

	dsn := JSONStr(jparam, "dsn")
	// db_user:db_pwd@tcp(localhost:3306)/my_db?charset=utf8mb4&parseTime=true&loc=Local&timeout=10s&writeTimeout=10s&readTimeout=10s
	driverName := JSONStrOr(jparam, "driver", "mysql")
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return EvaluatorResult{Err: err}
	}

	dr := &DbReuse{DB: db, autoClose: !global}
	ctx.SetVar(key, dr)

	if global {
		SetDBReuse(key, dr)
	}

	return EvaluatorResult{Mode: EvaluatorDel, Key: key}
}

type DbReuse struct {
	*sql.DB
	autoClose bool
}

func (d *DbReuse) Close() error {
	if d.autoClose {
		return d.DB.Close()
	}
	return nil
}

func GetDBReuse(key string) (*DbReuse, bool) {
	globalDbLock.Lock()
	db, ok := globalDbMap[key]
	globalDbLock.Unlock()
	return db, ok
}

func SetDBReuse(key string, dr *DbReuse) {
	globalDbLock.Lock()
	globalDbMap[key] = dr
	globalDbLock.Unlock()
}

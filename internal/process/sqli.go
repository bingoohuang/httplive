package process

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bingoohuang/gg/pkg/emb"
	"github.com/bingoohuang/gg/pkg/sqx"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/gg/pkg/vars"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

func init() {
	registerHlHandlers("sqli", func() HlHandler { return &Sqli{} })
}

type Sqli struct {
	db             *sql.DB
	Query          string `json:"query"`
	DriverName     string `json:"driverName"`
	DataSourceName string `json:"dataSourceName"`

	Scripts []string `json:"scripts"`
}

func (s Sqli) HlHandle(c *gin.Context, apiModel *APIDataModel, asset func(name string) string) error {
	if c.Request.Method == "GET" {
		emb.ServeFile(subStatic, "sqli.html", c.Writer, c.Request)
		return nil
	}

	if s.db == nil {
		return fmt.Errorf("db is not initialized")
	}

	query := vars.EvalSubstitute(s.Query, vars.VarValueHandler(func(name, params, expr string) interface{} {
		if strings.HasPrefix(name, "query_") {
			q := name[len("query_"):]
			return c.Query(q)
		}
		return expr
	}))

	result, err := sqx.NewSQL(query).QueryAsMaps(s.db)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		c.JSON(200, err.Error())
	} else {
		c.JSON(200, result)
	}

	return nil
}

func (s *Sqli) AfterUnmashal() {
	driverName := ss.Or(s.DriverName, "sqlite")
	dataSourceName := s.DataSourceName

	if driverName == "sqlite" {
		dir, err := os.MkdirTemp("", "sqlite-")
		if err != nil {
			log.Printf("E! create temp dir failed: %v", err)
			return
		}

		// defer os.RemoveAll(dir)
		dataSourceName = filepath.Join(dir, "db")
	}

	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		log.Printf("E! open %s failed: %v", driverName, err)
		return
	}

	s.db = db

	for _, script := range s.Scripts {
		if _, err := db.Exec(script); err != nil {
			log.Printf("E! execute %s failed: %v", script, err)
			continue
		}
	}
}

// docker run -p 3306:3306 --name mysql -e MYSQL_ROOT_PASSWORD=root -d mysql:5.7.37

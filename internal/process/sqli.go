package process

import (
	"database/sql"
	"fmt"
	"github.com/bingoohuang/gg/pkg/emb"
	"github.com/bingoohuang/gg/pkg/sqx"
	"github.com/bingoohuang/gg/pkg/vars"
	"github.com/gin-gonic/gin"
	"io/fs"
	"log"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	registerHlHandlers("sqli", func() HlHandler { return &Sqli{} })
}

type Sqli struct {
	Scripts []string `json:"scripts"`
	Query   string   `json:"query"`

	db *sql.DB
}

func (s Sqli) HlHandle(c *gin.Context, apiModel *APIDataModel, asset func(name string) string) error {
	if c.Request.Method == "GET" {
		data := emb.AssetBytes(fs.FS(staticFS), "static/sqli.html", false)
		c.Data(200, "text/html; charset=utf-8", data)
		return nil
	}

	if s.db == nil {
		return fmt.Errorf("db is not initialized")
	}

	query := vars.EvalSubstitute(s.Query, vars.VarValueHandler(func(name string) interface{} {
		if strings.HasPrefix(name, "query_") {
			q := name[len("query_"):]
			return c.Query(q)
		}
		return ""
	}))

	result, err := sqx.NewSQL(query).QueryAsMap(s.db)
	if err != nil && err != sql.ErrNoRows {
		c.JSON(200, err.Error())
	} else {
		c.JSON(200, result)
	}

	return nil
}

func (s *Sqli) AfterUnmashal() {
	dir, err := os.MkdirTemp("", "test-")
	if err != nil {
		log.Printf("E! create temp dir failed: %v", err)
		return
	}

	//defer os.RemoveAll(dir)

	fn := filepath.Join(dir, "db")
	db, err := sql.Open("sqlite", fn)
	if err != nil {
		log.Printf("E! open sqlite %s failed: %v", fn, err)
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

package process

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/gin-gonic/gin"
)

const (
	HlServerStatic = "serverStatic"
)

type ServeStatic struct {
	Root      string `json:"root"`
	AutoIndex bool   `json:"auto_index"`
	Grid      bool   `json:"grid"`
	Index     string `json:"index"`
}

var DirListTemplate *template.Template
var GridTemplate *template.Template

func (m ServeStatic) Handle(c *gin.Context, apiModel *APIDataModel) error {
	rootStat, err := os.Stat(m.Root)
	if err != nil {
		return fmt.Errorf("root directory: %w", err)
	}

	if !rootStat.IsDir() { // not a directory
		c.File(m.Root)
		return nil
	}

	format := c.Query("format")
	grid := format == "grid"
	if format == "list" && m.AutoIndex {
		m.Grid = false
	}

	urlPath := c.Request.URL.Path
	fixPath, _ := ParsePathParams(apiModel)
	urlPath = strings.TrimPrefix(urlPath, fixPath)
	if urlPath == "" || urlPath == "/" {
		if grid && m.AutoIndex {
			return m.listPage(c, m.Root, grid)
		}
		if format == "list" && m.AutoIndex {
			return m.listPage(c, m.Root, false)
		}

		if m.Index != "" {
			c.File(path.Join(m.Root, m.Index))
		} else if m.AutoIndex {
			return m.listPage(c, m.Root, grid)
		} else {
			c.Status(http.StatusNotFound)
		}
		return nil
	}

	f := path.Join(m.Root, urlPath)
	if fstat, err := os.Stat(f); err != nil {
		return fmt.Errorf("root directory: %w", err)
	} else if fstat.IsDir() {
		return m.listPage(c, f, grid)
	} else {
		c.File(f)
	}

	return nil
}

func (m ServeStatic) listPage(c *gin.Context, dir string, grid bool) error {
	data, err := ListDir(dir, c.Request.URL.RawQuery, 1000)
	if err != nil {
		return err
	}
	c.Header("Content-Type", "text/html; charset=utf-8")

	if m.Grid || grid {
		var imageFiles []File
		for _, d := range data.Files {
			name := strings.ToLower(d.Name)
			if ss.HasSuffix(name, ".jpg", ".jpeg", ".png") {
				imageFiles = append(imageFiles, d)
			}
		}

		if len(imageFiles) > 0 {
			data.Files = imageFiles
			return GridTemplate.Execute(c.Writer, data)
		}

	}
	return DirListTemplate.Execute(c.Writer, data)
}

func ParsePathParams(apiModel *APIDataModel) (prefix string, hasParams bool) {
	segments := strings.Split(apiModel.Endpoint, "/")
	for i, seg := range segments {
		if ss.HasPrefix(seg, "*", ":") {
			return strings.Join(segments[:i], "/"), true
		}
	}
	prefix = strings.Join(segments, "/")
	return apiModel.Endpoint, false
}

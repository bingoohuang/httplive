package process

import (
	"errors"
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
	Root     string `json:"root"`
	Dir      string `json:"dir"` // (empty) / list / grid
	Index    string `json:"index"`
	dirFirst bool
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

	if dir := c.Query("dir"); dir != "" {
		m.dirFirst = true
		m.Dir = dir
	}

	urlPath := c.Request.URL.Path
	fixPath, _ := ParsePathParams(apiModel)
	urlPath = strings.TrimPrefix(urlPath, fixPath)
	if urlPath == "" || urlPath == "/" {
		if m.Index != "" {
			indexFile := path.Join(m.Root, m.Index)
			if indexFileStat, err := os.Stat(indexFile); err != nil || indexFileStat.IsDir() {
				m.Index = ""
			}
		}

		if !m.dirFirst && m.Index != "" {
			c.File(path.Join(m.Root, m.Index))
			return nil
		}

		switch m.Dir {
		case "grid":
			return m.listPage(c, m.Root)
		case "list":
			return m.listPage(c, m.Root)
		}

		if m.Index != "" {
			c.File(path.Join(m.Root, m.Index))
		} else {
			c.Status(http.StatusNotFound)
		}
		return nil
	}

	f := path.Join(m.Root, urlPath)
	if fstat, err := os.Stat(f); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.Status(http.StatusNotFound)
			return nil
		}
		return fmt.Errorf("root directory: %w", err)
	} else if fstat.IsDir() {
		return m.listPage(c, f)
	} else {
		c.File(f)
	}

	return nil
}

func (m ServeStatic) listPage(c *gin.Context, dir string) error {
	data, err := ListDir(dir, c.Request.URL.RawQuery, 1000)
	if err != nil {
		return err
	}
	c.Header("Content-Type", "text/html; charset=utf-8")

	if m.Dir == "grid" {
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

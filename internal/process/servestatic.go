package process

import (
	"fmt"
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
	Index     string `json:"index"`
}

func (m ServeStatic) Handle(c *gin.Context, apiModel *APIDataModel) error {
	rootStat, err := os.Stat(m.Root)
	if err != nil {
		return fmt.Errorf("root directory: %w", err)
	}

	if !rootStat.IsDir() { // not a directory
		c.File(m.Root)
		return nil
	}

	urlPath := c.Request.URL.Path
	fixPath, _ := ParsePathParams(apiModel)
	urlPath = strings.TrimPrefix(urlPath, fixPath)
	if urlPath == "" || urlPath == "/" {
		if m.Index != "" {
			c.File(path.Join(m.Root, m.Index))
		} else if m.AutoIndex {
			return ListDir(c.Writer, m.Root, 1000)
		} else {
			c.Status(http.StatusNotFound)
		}
		return nil
	}

	f := path.Join(m.Root, urlPath)
	if fstat, err := os.Stat(f); err != nil {
		return fmt.Errorf("root directory: %w", err)
	} else if fstat.IsDir() {
		return ListDir(c.Writer, f, 1000)
	} else {
		c.File(f)
	}

	return nil
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

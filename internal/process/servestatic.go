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
	endpoint := apiModel.Endpoint
	segments := strings.Split(endpoint, "/")
	for i, seg := range segments {
		if ss.HasPrefix(seg, "*", ":") {
			segments = segments[:i]
			break
		}
	}
	fixPath := strings.Join(segments, "/")
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

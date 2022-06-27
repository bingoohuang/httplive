package process

import (
	"strings"

	"github.com/bingoohuang/httplive/pkg/util"
	"github.com/gin-gonic/gin"
)

func TrimContextPath(c *gin.Context) string {
	p := c.Request.URL.Path
	if Envs.ContextPath != "/" {
		p = strings.TrimPrefix(p, Envs.ContextPath)
	}

	return util.Or(p, "/")
}

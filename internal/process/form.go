package process

import (
	"github.com/bingoohuang/gg/pkg/emb"
	"github.com/gin-gonic/gin"
)

func init() {
	registerHlHandlers("form", func() HlHandler { return &Form{} })
}

type Form struct{}

func (s Form) HlHandle(c *gin.Context, apiModel *APIDataModel, asset func(name string) string) error {
	if c.Request.Method == "GET" {
		emb.ServeFile(subStatic, "form.html", c.Writer, c.Request)
		return nil
	}

	c.Request.ParseMultipartForm(32 * 1024 * 1024) // 32M
	c.Request.ParseForm()

	c.PureJSON(200, map[string]interface{}{
		"Status": "OK",
		"Form":   c.Request.Form,
	})

	return nil
}

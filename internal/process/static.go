package process

import (
	"embed"
	"io/fs"

	"github.com/bingoohuang/gg/pkg/emb"
	"github.com/bingoohuang/gg/pkg/fp"
	"github.com/gin-gonic/gin"
)

var (
	//go:embed _static
	staticFS  embed.FS
	subStatic = fp.Must(fs.Sub(staticFS, "_static"))
)

// ServeStaticFS deal processing static files.
func ServeStaticFS(c *gin.Context, path string) {
	emb.ServeFile(subStatic, path, c.Writer, c.Request)
}

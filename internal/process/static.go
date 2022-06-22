package process

import (
	"embed"
	"github.com/bingoohuang/gg/pkg/emb"
	"github.com/gin-gonic/gin"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed static
var staticFS embed.FS

// ServeStaticFS deal processing static files.
func ServeStaticFS(c *gin.Context, path string) {
	path = strings.TrimPrefix(path, "/")
	ServeFile(fs.FS(staticFS), path, c.Writer, c.Request)
}

func ServeFile(f fs.FS, name string, w http.ResponseWriter, r *http.Request) {
	data, hash, contentType, err := emb.Asset(f, name, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("If-None-Match") == hash {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Add("Cache-Control", "public, max-age=31536000")
	w.Header().Add("ETag", hash)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

package res

import (
	"bytes"
	"embed"
	"io/fs"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// TryGetFile ...
func TryGetFile(publicFS embed.FS, c *gin.Context, assetPath, contextPath string) bool {
	if os.Getenv("debug") != "" {
		return tryGetLocalFile(c, assetPath, contextPath)
	}

	return tryGetAssetFile(publicFS, c, assetPath, contextPath)
}

func tryGetLocalFile(c *gin.Context, filePath, contextPath string) bool {
	workingDir, _ := os.Getwd()
	f := path.Join(workingDir, filePath)
	if _, err := os.Stat(f); err != nil {
		return false
	}

	ext := path.Ext(filePath)
	contentType := mime.TypeByExtension(ext)
	fileData, _ := ioutil.ReadFile(f)
	c.Data(http.StatusOK, contentType, ReplaceContextPath(fileData, contextPath))
	return true
}

func tryGetAssetFile(publicFS embed.FS, c *gin.Context, filePath, contextPath string) bool {
	filePath = strings.TrimPrefix(filePath, "/")
	info, err := fs.Stat(publicFS, filePath)
	if err != nil || info.IsDir() {
		return false
	}

	// 具体单个文件，直接查找静态文件，返回文件内容
	if err := serveStaticFile(publicFS, c, filePath, contextPath); err != nil {
		_ = c.Error(err)
	}

	return true
}

func serveStaticFile(publicFS embed.FS, c *gin.Context, filePath, contextPath string) error {
	buf, err := fs.ReadFile(publicFS, filePath)
	if err != nil {
		return err
	}

	ext := path.Ext(filePath)
	contentType := mime.TypeByExtension(ext)

	if strings.EqualFold(ext, ".js") || strings.EqualFold(ext, ".css") || strings.EqualFold(ext, ".html") {
		c.Data(http.StatusOK, contentType, ReplaceContextPath(buf, contextPath))
	} else {
		c.Data(http.StatusOK, contentType, buf)
	}

	return nil
}

const (
	contextPathPlaceholder      = "${ContextPath}"
	contextPathSlashPlaceholder = "${ContextPathSlash}"
)

func ReplaceContextPath(data []byte, contextPath string) []byte {
	if contextPath == "/" {
		data = bytes.ReplaceAll(data, []byte(contextPathPlaceholder), []byte(""))
		return bytes.ReplaceAll(data, []byte(contextPathSlashPlaceholder), []byte("/"))
	}

	data = bytes.ReplaceAll(data, []byte(contextPathPlaceholder), []byte(contextPath))
	return bytes.ReplaceAll(data, []byte(contextPathSlashPlaceholder), []byte(contextPath+"/"))
}

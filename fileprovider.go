package httplive

import (
	"bytes"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/markbates/pkger"
)

// TryGetFile ...
func TryGetFile(c *gin.Context, assetPath string) bool {
	if os.Getenv("debug") != "" {
		return tryGetLocalFile(c, assetPath)
	}

	return tryGetAssetFile(c, assetPath)
}

func tryGetLocalFile(c *gin.Context, filePath string) bool {
	f := path.Join(Environments.WorkingDir, filePath)
	if _, err := os.Stat(f); err != nil {
		return false
	}

	ext := path.Ext(filePath)
	contentType := mime.TypeByExtension(ext)
	fileData, _ := ioutil.ReadFile(f)
	c.Data(http.StatusOK, contentType, replaceContextPath(fileData))
	return true
}

func tryGetAssetFile(c *gin.Context, filePath string) bool {
	pkger.Include("/public") // nolint:staticcheck
	info, err := pkger.Stat(filePath)
	if err != nil || info.IsDir() {
		return false
	}

	// 具体单个文件，直接查找静态文件，返回文件内容
	if err := serveStaticFile(c, filePath); err != nil {
		_ = c.Error(err)
	}

	return true
}

func serveStaticFile(c *gin.Context, filePath string) error {
	f, err := pkger.Open(filePath)
	if err != nil {
		return err
	}

	defer f.Close()

	ext := path.Ext(filePath)
	contentType := mime.TypeByExtension(ext)
	buf := new(bytes.Buffer)
	_, _ = io.Copy(buf, f)

	if strings.EqualFold(ext, ".js") || strings.EqualFold(ext, ".css") || strings.EqualFold(ext, ".html") {
		c.Data(http.StatusOK, contentType, replaceContextPath(buf.Bytes()))
	} else {
		c.Data(http.StatusOK, contentType, buf.Bytes())
	}

	return nil
}

const (
	contextPathPlaceholder      = "${ContextPath}"
	contextPathSlashPlaceholder = "${ContextPathSlash}"
)

func replaceContextPath(data []byte) []byte {
	if Environments.ContextPath == "/" {
		data = bytes.ReplaceAll(data, []byte(contextPathPlaceholder), []byte(""))
		return bytes.ReplaceAll(data, []byte(contextPathSlashPlaceholder), []byte("/"))
	}

	data = bytes.ReplaceAll(data, []byte(contextPathPlaceholder), []byte(Environments.ContextPath))
	return bytes.ReplaceAll(data, []byte(contextPathSlashPlaceholder), []byte(Environments.ContextPath+"/"))
}

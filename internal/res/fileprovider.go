package res

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
func TryGetFile(c *gin.Context, assetPath, contextPath string) bool {
	if os.Getenv("debug") != "" {
		return tryGetLocalFile(c, assetPath, contextPath)
	}

	return tryGetAssetFile(c, assetPath, contextPath)
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

func tryGetAssetFile(c *gin.Context, filePath, contextPath string) bool {
	pkger.Include("/public") // nolint:staticcheck
	info, err := pkger.Stat(filePath)
	if err != nil || info.IsDir() {
		return false
	}

	// 具体单个文件，直接查找静态文件，返回文件内容
	if err := serveStaticFile(c, filePath, contextPath); err != nil {
		_ = c.Error(err)
	}

	return true
}

func serveStaticFile(c *gin.Context, filePath, contextPath string) error {
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
		c.Data(http.StatusOK, contentType, ReplaceContextPath(buf.Bytes(), contextPath))
	} else {
		c.Data(http.StatusOK, contentType, buf.Bytes())
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

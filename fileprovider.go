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
	"github.com/sirupsen/logrus"
)

// TryGetFile ...
func TryGetFile(c *gin.Context, assetPath string) bool {
	if os.Getenv("debug") != "" {
		return TryGetLocalFile(c, assetPath)
	}

	return TryGetAssetFile(c, assetPath)
}

// TryGetLocalFile ...
func TryGetLocalFile(c *gin.Context, filePath string) bool {
	logrus.Debugf("fs:dev local file for: %s", filePath)
	f := path.Join(Environments.WorkingDir, filePath)

	if _, err := os.Stat(f); err != nil {
		return false
	}

	ext := path.Ext(filePath)
	contentType := mime.TypeByExtension(ext)
	fileData, _ := ioutil.ReadFile(f)
	c.Data(http.StatusOK, contentType, ReplaceContextPath(fileData))
	return true
}

// TryGetAssetFile ...
func TryGetAssetFile(c *gin.Context, filePath string) bool {
	logrus.Debugf("fs:bindata asset try getfile executed for: %s", filePath)

	info, err := pkger.Stat(filePath)
	if err != nil || info.IsDir() {
		return false
	}

	// 具体单个文件，直接查找静态文件，返回文件内容
	if err := ServeStaticFile(c, filePath); err != nil {
		c.Status(http.StatusInternalServerError)
		c.Writer.Write([]byte(err.Error()))
	}

	return true
}

func ServeStaticFile(c *gin.Context, filePath string) error {
	f, err := pkger.Open(filePath)
	if err != nil {
		return err
	}

	defer f.Close()

	contentType := mime.TypeByExtension(path.Ext(filePath))
	buf := new(bytes.Buffer)
	io.Copy(buf, f)

	c.Data(http.StatusOK, contentType, ReplaceContextPath(buf.Bytes()))

	return nil
}

const contextPathPlaceholder = "${ContextPath}"
const contextPathSlashPlaceholder = "${ContextPathSlash}"

func ReplaceContextPathString(data string) string {
	if Environments.ContextPath == "/" {
		data = strings.ReplaceAll(data, contextPathPlaceholder, "")
		return strings.ReplaceAll(data, contextPathSlashPlaceholder, "/")
	}

	data = strings.ReplaceAll(data, contextPathPlaceholder, Environments.ContextPath)
	return strings.ReplaceAll(data, contextPathSlashPlaceholder, Environments.ContextPath+"/")
}
func ReplaceContextPath(data []byte) []byte {
	if Environments.ContextPath == "/" {
		data = bytes.ReplaceAll(data, []byte(contextPathPlaceholder), []byte(""))
		return bytes.ReplaceAll(data, []byte(contextPathSlashPlaceholder), []byte("/"))
	}

	data = bytes.ReplaceAll(data, []byte(contextPathPlaceholder), []byte(Environments.ContextPath))
	return bytes.ReplaceAll(data, []byte(contextPathSlashPlaceholder), []byte(Environments.ContextPath+"/"))
}

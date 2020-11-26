package httplive

import (
	"bytes"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
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

	if _, err := os.Stat(f); err == nil {
		ext := path.Ext(filePath)
		contentType := mime.TypeByExtension(ext)
		fileData, _ := ioutil.ReadFile(f)
		c.Data(http.StatusOK, contentType, ReplaceContextPath(fileData))
		return true
	}

	return false
}

// TryGetAssetFile ...
func TryGetAssetFile(c *gin.Context, filePath string) bool {
	logrus.Debugf("fs:bindata asset try getfile executed for: %s", filePath)
	assetData, err := Asset(filePath)

	if err == nil && assetData != nil {
		contentType := mime.TypeByExtension(path.Ext(filePath))
		c.Data(http.StatusOK, contentType, ReplaceContextPath(assetData))
		return true
	}

	return false
}

func ReplaceContextPathString(data string) string {
	if Environments.ContextPath == "/" {
		data = strings.ReplaceAll(data, "${ContextPath}", "")
		data = strings.ReplaceAll(data, "${ContextPathSlash}", "/")
	} else {
		data = strings.ReplaceAll(data, "${ContextPath}", Environments.ContextPath)
		data = strings.ReplaceAll(data, "${ContextPathSlash}", Environments.ContextPath+"/")
	}

	return data
}
func ReplaceContextPath(data []byte) []byte {
	if Environments.ContextPath == "/" {
		data = bytes.ReplaceAll(data, []byte("${ContextPath}"), []byte(""))
		data = bytes.ReplaceAll(data, []byte("${ContextPathSlash}"), []byte("/"))
	} else {
		data = bytes.ReplaceAll(data, []byte("${ContextPath}"), []byte(Environments.ContextPath))
		data = bytes.ReplaceAll(data, []byte("${ContextPathSlash}"), []byte(Environments.ContextPath+"/"))
	}

	return data
}

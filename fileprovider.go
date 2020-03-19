package httplive

import (
	"mime"
	"net/http"
	"os"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

// TryGetLocalFile ...
func TryGetLocalFile(c *gin.Context, filePath string) {
	logrus.Debugf("fs:dev local file for: %s", filePath)
	f := path.Join(Environments.WorkingDir, filePath)

	if _, err := os.Stat(f); err == nil {
		c.File(f)
		c.Abort()
	}
}

// TryGetAssetFile ...
func TryGetAssetFile(c *gin.Context, filePath string) {
	logrus.Debugf("fs:bindata asset trygetfile executed for: %s", filePath)
	assetData, err := Asset(filePath)

	if err == nil && assetData != nil {
		ext := path.Ext(filePath)
		contentType := mime.TypeByExtension(ext)
		c.Data(http.StatusOK, contentType, assetData)
		c.Abort()
	}
}

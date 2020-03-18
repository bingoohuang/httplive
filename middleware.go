package httplive

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware ...
// nolint lll
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Max-Age", "86400")
		h.Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		h.Set("Access-Control-Allow-Headers", "X-Requested-With, Content-Type, Origin, Authorization, Accept, Client-Security-Token, Accept-Encoding, x-access-token")
		h.Set("Access-Control-Expose-Headers", "Content-Length")
		h.Set("Access-Control-Allow-Credentials", "true")
		h.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		h.Set("Pragma", "no-cache")
		h.Set("Expires", "0")

		if c.Request.Method == "OPTIONS" {
			fmt.Println("OPTIONS")
			c.AbortWithStatus(http.StatusOK)
		} else {
			c.Next()
		}
	}
}

// StaticFileMiddleware ...
func StaticFileMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		uriPath := c.Request.URL.Path
		method := c.Request.Method
		assetPath := "public" + uriPath
		ext := path.Ext(assetPath)

		if method == "GET" && uriPath == "/" {
			assetPath = "public/index.html"
		}

		if ext == ".map" {
			c.Status(http.StatusNotFound)
			c.Abort()

			return
		}

		fp := os.Getenv("debug")
		if fp != "" {
			TryGetLocalFile(c, assetPath)
		} else {
			TryGetAssetFile(c, assetPath)
		}

		if c.IsAborted() {
			return
		}

		c.Next()
	}
}

// APIMiddleware ...
func APIMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		key := CreateEndpointKey(method, c.Request.URL.Path)
		model, err := GetEndpoint(key)

		if err != nil {
			Broadcast(c)
			c.JSON(http.StatusNotFound, err)
			c.Abort()

			return
		}

		if model != nil {
			if model.MimeType != "" {
				reader := bytes.NewReader(model.FileContent)
				http.ServeContent(c.Writer, c.Request, model.Filename, time.Now(), reader)
				c.Abort()

				return
			}

			Broadcast(c)

			var body interface{}

			_ = json.Unmarshal([]byte(model.Body), &body)

			c.JSON(http.StatusOK, body)
			c.Abort()

			return
		}

		c.Next()
	}
}

// ConfigJsMiddleware ...
func ConfigJsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/config.js" {
			fileContent := "define('config', { defaultPort:'" + Environments.DefaultPort + "', savePath: '/webcli/api/save', " +
				"fetchPath: '/webcli/api/endpoint', deletePath: '/webcli/api/deleteendpoint', " +
				"treePath: '/webcli/api/tree', componentId: ''});"
			c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileContent)))
			c.Writer.Header().Set("Content-Type", "application/javascript")
			c.String(http.StatusOK, fileContent)
			c.Abort()

			return
		}

		c.Next()
	}
}

package util

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// CORSMiddleware ...
func CORSMiddleware(c *gin.Context) {
	h := c.Header
	h("Access-Control-Allow-Origin", "*")
	h("Access-Control-Max-Age", "86400")
	h("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
	h("Access-Control-Allow-Headers", "X-Requested-With, Content-Type, "+
		"Origin, Authorization, Accept, Client-Security-Token, Accept-Encoding, x-access-token")
	h("Access-Control-Expose-Headers", "Content-Length")
	h("Access-Control-Allow-Credentials", "true")
	h("Cache-Control", "no-cache, no-store, must-revalidate")
	h("Pragma", "no-cache")
	h("Expires", "0")

	if c.Request.Method == http.MethodOptions {
		logrus.Infof(http.MethodOptions)
		c.AbortWithStatus(http.StatusOK)
		return
	}

	c.Next()
}

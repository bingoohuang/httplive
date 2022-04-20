package process

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bingoohuang/jj"
	"github.com/gin-gonic/gin"
)

/*
"_auth": {
   "basicAuth": "user:pass",
   "bearerToken": "token",
   "apiKey": {
      "key": "y-develop-id",
      "value": "123",
      "header": true,
      "queryParams": false
   }
}
*/

type ApiKey struct {
	Key         string `json:"key,omitempty"`
	Value       string `json:"value,omitempty"`
	Header      bool   `json:"header,omitempty"`
	QueryParams bool   `json:"queryParams,omitempty"`
}

func (k *ApiKey) Auth(c *gin.Context) bool {
	if k.Key == "" {
		return true
	}

	var val string
	if k.Header {
		val = c.GetHeader(http.CanonicalHeaderKey(k.Key))
	} else if k.QueryParams {
		val = c.Query(k.Key)
	} else {
		val = c.GetHeader(http.CanonicalHeaderKey(k.Key))
		if val == "" {
			val = c.Query(k.Key)
		}
	}

	if val == k.Value {
		return true
	}
	c.AbortWithStatus(http.StatusUnauthorized)
	return false
}

type AuthRequest interface {
	AuthRequest(c *gin.Context) bool
}

type passAuthRequest struct{}

func (a *passAuthRequest) AuthRequest(*gin.Context) bool { return true }

func ParseAuth(body string) (string, AuthRequest) {
	var auth AuthorizationWrap
	_ = json.Unmarshal([]byte(body), &auth)
	if auth.Auth != nil {
		body, _ = jj.Delete(body, "_auth")
		body, _ = jj.Delete(body, "_hl")
		return body, auth.Auth
	}

	return body, &passAuthRequest{}
}

type AuthorizationWrap struct {
	Auth *Authorization `json:"_auth,omitempty"`
}

type Authorization struct {
	BasicAuth   string  `json:"basicAuth,omitempty"`   // Authorization: Basic base64encode(username+":"+password)
	BearerToken string  `json:"bearerToken,omitempty"` // Authorization: Bearer <token>
	ApiKey      *ApiKey `json:"apiKey,omitempty"`
}

func (a *Authorization) AuthRequest(c *gin.Context) bool {
	if a.BasicAuth != "" {
		return a.checkBasicAuth(c)
	}

	if a.BearerToken != "" {
		return a.checkBearerToken(c)
	}

	if a.ApiKey != nil {
		return a.ApiKey.Auth(c)
	}

	return true
}

func (a *Authorization) checkBasicAuth(c *gin.Context) bool {
	h := c.GetHeader("Authorization")
	b := "Basic " + base64.StdEncoding.EncodeToString([]byte(a.BasicAuth))
	if h == b {
		return true
	}
	c.Header("WWW-Authenticate", "Basic realm="+strconv.Quote("Authorization Required"))
	c.AbortWithStatus(http.StatusUnauthorized)
	return false
}

func (a *Authorization) checkBearerToken(c *gin.Context) bool {
	h := c.GetHeader("Authorization")
	if h == "Bearer "+a.BearerToken {
		return true
	}
	c.AbortWithStatus(http.StatusUnauthorized)
	return false
}

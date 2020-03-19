package httplive

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// CreateEndpointKey ...
func CreateEndpointKey(method string, endpoint string) string {
	return strings.ToLower(method + endpoint)
}

// Broadcast ...
func Broadcast(c *gin.Context) {
	msg := WsMessage{
		Time:   time.Now().Format("2006-01-02 15:04:05.000"),
		Host:   c.Request.Host,
		Body:   GetRequestBody(c),
		Method: c.Request.Method,
		Path:   c.Request.URL.Path,
		Query:  c.Request.URL.Query(),
		Header: GetHeaders(c),
	}

	for id, conn := range Clients {
		if err := conn.WriteJSON(msg); err != nil {
			logrus.Warnf("error: %v", err)

			conn.Close()

			delete(Clients, id)
		}
	}
}

// GetHeaders ...
func GetHeaders(c *gin.Context) map[string]string {
	hdr := make(map[string]string, len(c.Request.Header))
	for k, v := range c.Request.Header {
		hdr[k] = v[0]
	}

	return hdr
}

// GetIP ...
func GetIP(c *gin.Context) string {
	ip := c.ClientIP()
	return ip
}

// GetMultiPartFormValue ...
func GetMultiPartFormValue(c *gin.Context) interface{} {
	var requestBody interface{}

	multipartForm := make(map[string]interface{})

	_ = c.Request.ParseMultipartForm(32 * 1024 * 1024) // nolint gomnd 32M

	if c.Request.MultipartForm != nil {
		for key, values := range c.Request.MultipartForm.Value {
			multipartForm[key] = strings.Join(values, "")
		}

		for key, file := range c.Request.MultipartForm.File {
			for k, f := range file {
				formKey := fmt.Sprintf("%s%d", key, k)
				multipartForm[formKey] = map[string]interface{}{"filename": f.Filename, "size": f.Size}
			}
		}

		if len(multipartForm) > 0 {
			requestBody = multipartForm
		}
	}

	return requestBody
}

// GetFormBody ...
func GetFormBody(c *gin.Context) interface{} {
	var requestBody interface{}

	form := make(map[string]string)

	_ = c.Request.ParseForm()

	for key, values := range c.Request.PostForm {
		form[key] = strings.Join(values, "")
	}

	if len(form) > 0 {
		requestBody = form
	}

	return requestBody
}

// TryBind ...
func TryBind(c *gin.Context) interface{} {
	var model interface{}
	err := c.Bind(&model)

	if err != nil {
		return nil
	}

	return model
}

// GetRequestBody ...
func GetRequestBody(c *gin.Context) interface{} {
	multiPartFormValue := GetMultiPartFormValue(c)
	if multiPartFormValue != nil {
		return multiPartFormValue
	}

	formBody := GetFormBody(c)
	if formBody != nil {
		return formBody
	}

	contentType := c.ContentType()
	method := c.Request.Method

	if method == http.MethodGet {
		return nil
	}

	switch contentType {
	case binding.MIMEJSON:
		return TryBind(c)
	case binding.MIMEXML, binding.MIMEXML2:
		body, err := ioutil.ReadAll(c.Request.Body)
		if err == nil {
			return string(body)
		}

		return nil
	default:
		return nil
	}
}

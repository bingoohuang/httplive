package httplive

import (
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/bingoohuang/gor/giu"
	"github.com/boltdb/bolt"
	"github.com/gin-gonic/gin"
)

func createJsTreeModel(a APIDataModel) JsTreeDataModel {
	model := JsTreeDataModel{
		ID:        a.ID,
		OriginKey: CreateEndpointKey(a.Method, a.Endpoint),
		Key:       a.Endpoint,
		Text:      a.Endpoint,
		Children:  []JsTreeDataModel{},
	}

	m := a.Method
	switch m {
	case http.MethodPost, http.MethodPut, http.MethodDelete:
	default:
		m = http.MethodGet
	}

	httpMethodLabelMap := map[string]string{
		http.MethodGet:    "label label-primary label-small",
		http.MethodPost:   "label label-success label-small",
		http.MethodPut:    "label label-warning label-small",
		http.MethodDelete: "label label-danger label-small",
	}

	model.Type = m
	model.Text = fmt.Sprintf(`<span class="%v">%v</span> %v`, httpMethodLabelMap[m], m, a.Endpoint)

	return model
}

type treeT struct {
	giu.T `url:"GET /api/tree"`
}

// Tree ...
func (ctrl WebCliController) Tree(c *gin.Context, _ treeT) {
	apis := EndpointList()
	trees := make([]JsTreeDataModel, len(apis))

	for i, api := range apis {
		trees[i] = createJsTreeModel(api)
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       "0",
		"key":      "APIs",
		"text":     "APIs",
		"state":    map[string]interface{}{"opened": true},
		"children": trees,
		"type":     "root",
	})
}

type backupT struct {
	giu.T `url:"GET /api/backup"`
}

// Backup ...
func (ctrl WebCliController) Backup(c *gin.Context, _ backupT) error {
	db := OpenDB()
	defer db.Close()

	return db.View(func(tx *bolt.Tx) error {
		c.Writer.Header().Set("Content-Type", "application/octet-stream")
		c.Writer.Header().Set("Content-Disposition", `attachment; filename="httplive.db"`)
		c.Writer.Header().Set("Content-Length", strconv.Itoa(int(tx.Size())))
		_, err := tx.WriteTo(c.Writer)
		return err
	})
}

type downloadFileT struct {
	giu.T `url:"GET /api/downloadfile"`
}

// DownloadFile ...
func (ctrl WebCliController) DownloadFile(c *gin.Context, _ downloadFileT) {
	query := c.Request.URL.Query()
	endpoint := query.Get("endpoint")

	if endpoint != "" {
		key := CreateEndpointKey(http.MethodGet, endpoint)
		model, err := GetEndpoint(key)

		if err == nil && model != nil {
			if model.MimeType != "" {
				c.Writer.Header().Set("Content-Disposition", `attachment; filename="`+model.Filename+`"`)

				c.Data(http.StatusOK, model.MimeType, model.FileContent)

				return
			}
		}
	}

	c.Status(http.StatusNotFound)
}

type endpointT struct {
	giu.T `url:"GET /api/endpoint"`
}

// Endpoint ...
func (ctrl WebCliController) Endpoint(c *gin.Context, _ endpointT) {
	query := c.Request.URL.Query()
	endpoint := query.Get("endpoint")
	method := query.Get("method")

	if endpoint == "" || method == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "endpoint and method required"})
		return
	}

	key := CreateEndpointKey(method, endpoint)
	model, _ := GetEndpoint(key)

	c.JSON(http.StatusOK, model)
}

type saveT struct {
	giu.T `url:"POST /api/save"`
}

// Save ...
func (ctrl WebCliController) Save(model APIDataModel, c *gin.Context, _ saveT) {
	if err := SaveEndpoint(model); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": "ok"})
}

type saveEndpointT struct {
	giu.T `url:"POST /api/saveendpoint"`
}

// SaveEndpoint ...
func (ctrl WebCliController) SaveEndpoint(model EndpointModel, c *gin.Context, _ saveEndpointT) {
	mimeType, filename, fileContent, abort := parseFileContent(c, model)
	if abort {
		return
	}

	if key := model.OriginKey; key != "" {
		if updateEndpoint(c, model, key, mimeType, filename, fileContent) {
			return
		}
	} else {
		if newEndpoint(c, model, filename, mimeType, fileContent) {
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": "ok"})
}

func newEndpoint(c *gin.Context, model EndpointModel, filename, mimeType string, fileContent []byte) bool {
	endpoint := APIDataModel{
		Endpoint:    model.Endpoint,
		Method:      model.Method,
		Filename:    filename,
		MimeType:    mimeType,
		FileContent: fileContent}

	if filename != "" {
		if strings.HasSuffix(endpoint.Endpoint, "/") {
			endpoint.Endpoint += filename
		} else {
			endpoint.Endpoint += "/" + filename
		}
	}

	err := SaveEndpoint(endpoint)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		c.Abort()

		return true
	}

	return false
}

func updateEndpoint(c *gin.Context, model EndpointModel, key, mimeType, filename string, fileContent []byte) bool {
	endpoint, _ := GetEndpoint(key)
	if endpoint == nil {
		return false
	}

	endpoint.Method = model.Method
	endpoint.Endpoint = model.Endpoint
	endpoint.MimeType = mimeType
	endpoint.FileContent = fileContent
	endpoint.Filename = filename

	if filename != "" {
		if !strings.HasSuffix(endpoint.Endpoint, "/") {
			endpoint.Endpoint += "/"
		}

		endpoint.Endpoint += filename
	}

	_ = DeleteEndpoint(key)

	if err := SaveEndpoint(*endpoint); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		c.Abort()

		return true
	}

	return false
}

func parseFileContent(c *gin.Context, model EndpointModel) (mimeType, filename string, fileContent []byte, abort bool) {
	if model.IsFileResult {
		file, err := c.FormFile("file")
		if err != nil || file == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()

			return "", "", nil, true
		}

		f, _ := file.Open()
		fileContent, _ = ioutil.ReadAll(f)
		mimeType = mime.TypeByExtension(path.Ext(file.Filename))
		filename = file.Filename
	}

	return mimeType, filename, fileContent, false
}

type deleteEndpointT struct {
	giu.T `url:"GET /api/deleteendpoint"`
}

// DeleteEndpoint ...
func (ctrl WebCliController) DeleteEndpoint(c *gin.Context, _ deleteEndpointT) {
	method, endpoint := c.Query("method"), c.Query("endpoint")

	if endpoint == "" || method == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "endpoint and method required"})
		return
	}

	_ = DeleteEndpoint(CreateEndpointKey(method, endpoint))

	c.JSON(http.StatusOK, gin.H{"success": "ok"})
}

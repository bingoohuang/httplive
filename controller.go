package httplive

import (
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"path"
	"path/filepath"

	"github.com/bingoohuang/gor/giu"
	"github.com/gin-gonic/gin"
)

func createJsTreeModel(a APIDataModel) JsTreeDataModel {
	model := JsTreeDataModel{
		ID:        a.ID.Int(),
		OriginKey: CreateEndpointKey(a.Method, a.Endpoint),
		Key:       a.Endpoint,
		Text:      a.Endpoint,
		Children:  []JsTreeDataModel{},
	}

	label := ""
	switch a.Method {
	case http.MethodGet:
		label = "label label-primary label-small"
	case http.MethodPost:
		label = "label label-success label-small"
	case http.MethodPut:
		label = "label label-warning label-small"
	case http.MethodDelete:
		label = "label label-danger label-small"
	default:
		label = "label label-default label-small"
	}

	model.Type = a.Method
	model.Text = fmt.Sprintf(`<span class="%v">%v</span> %v`, label, a.Method, a.Endpoint)

	return model
}

type versionT struct {
	giu.T `url:"GET /version"`
}

const (
	// Version is the version x.y.z.
	Version = "1.0.4"
	// UpdateTime is the update time.
	UpdateTime = "2020-12-04 09:32:07"
)

// Version returns version information.
func (ctrl WebCliController) Version(_ versionT) gin.H {
	return gin.H{
		"version":    Version,
		"updateTime": UpdateTime,
	}
}

type treeT struct {
	giu.T `url:"GET /api/tree"`
}

// Tree return the api tree.
func (ctrl WebCliController) Tree(_ treeT) gin.H {
	apis := EndpointList(true)
	trees := make([]JsTreeDataModel, len(apis))

	for i, api := range apis {
		trees[i] = createJsTreeModel(api)
	}

	return gin.H{
		"id":       "0",
		"key":      "APIs",
		"text":     "APIs",
		"state":    gin.H{"opened": true},
		"children": trees,
		"type":     "root",
	}
}

type backupT struct {
	giu.T `url:"GET /api/backup"`
}

// Backup ...
func (ctrl WebCliController) Backup(c *gin.Context, _ backupT) {
	c.Header("Content-Disposition", `attachment; filename="`+filepath.Base(Environments.DBFile)+`"`)

	http.ServeFile(c.Writer, c.Request, Environments.DBFile)
}

type downloadFileT struct {
	giu.T `url:"GET /api/downloadfile"`
}

// DownloadFile ...
func (ctrl WebCliController) DownloadFile(c *gin.Context, _ downloadFileT) error {
	id, _ := c.GetQuery("id")
	model, err := GetEndpoint(ID(id))
	if err != nil {
		return err
	}

	if model != nil {
		c.Header("Content-Disposition", `attachment; filename="`+model.Filename+`"`)
		c.Data(http.StatusOK, model.MimeType, model.FileContent)

		return nil
	}

	c.Status(http.StatusNotFound)

	return nil
}

type endpointT struct {
	giu.T `url:"GET /api/endpoint"`
}

// Endpoint ...
func (ctrl WebCliController) Endpoint(c *gin.Context, _ endpointT) (giu.HTTPStatus, interface{}, error) {
	id := c.Query("id")
	model, err := GetEndpoint(ID(id))
	if err != nil {
		return giu.HTTPStatus(http.StatusInternalServerError), gin.H{"error": err.Error()}, err
	}

	if model != nil {
		return giu.HTTPStatus(http.StatusOK), model, nil
	}

	return giu.HTTPStatus(http.StatusNotFound), gin.H{"error": "endpoint and method required"}, nil
}

type saveT struct {
	giu.T `url:"POST /api/save"`
}

// Save 保存body.
func (ctrl WebCliController) Save(model APIDataModel, _ saveT) (giu.HTTPStatus, interface{}) {
	dp, err := SaveEndpoint(model)
	if err != nil {
		return giu.HTTPStatus(http.StatusBadRequest), gin.H{"error": err.Error()}
	}

	return giu.HTTPStatus(http.StatusOK), gin.H{"data": dp}
}

type saveEndpointT struct {
	giu.T `url:"POST /api/saveendpoint"`
}

// SaveEndpoint 保存路径、方法等变更.
func (ctrl WebCliController) SaveEndpoint(model APIDataModel, c *gin.Context, _ saveEndpointT) {
	mimeType, filename, fileContent := parseFileContent(c)
	if filename != "" {
		model.MimeType = mimeType
		model.Filename = filename
		model.Body = string(fileContent)
	}

	if dp, err := SaveEndpoint(model); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	} else {
		c.JSON(http.StatusOK, gin.H{"data": dp})
	}
}

func parseFileContent(c *gin.Context) (mimeType, filename string, fileContent []byte) {
	file, err := c.FormFile("file")
	if err != nil || file == nil {
		return mimeType, filename, fileContent
	}

	f, _ := file.Open()
	fileContent, _ = ioutil.ReadAll(f)
	mimeType = mime.TypeByExtension(path.Ext(file.Filename))
	filename = file.Filename

	return mimeType, filename, fileContent
}

type deleteEndpointT struct {
	giu.T `url:"GET /api/deleteendpoint"`
}

// DeleteEndpoint ...
func (ctrl WebCliController) DeleteEndpoint(c *gin.Context, _ deleteEndpointT) {
	id := c.Query("id")
	_ = DeleteEndpoint(id)

	c.JSON(http.StatusOK, gin.H{"success": "ok"})
}

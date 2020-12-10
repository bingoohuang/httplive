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

func (a APIDataModel) getLabelByMethod() string {
	switch a.Method {
	case http.MethodGet:
		return "label label-primary label-small"
	case http.MethodPost:
		return "label label-success label-small"
	case http.MethodPut:
		return "label label-warning label-small"
	case http.MethodDelete:
		return "label label-danger label-small"
	default:
		return "label label-default label-small"
	}
}

func (a APIDataModel) createJsTreeModel() JsTreeDataModel {
	model := JsTreeDataModel{
		ID:        a.ID.Int(),
		OriginKey: CreateEndpointKey(a.Method, a.Endpoint),
		Key:       a.Endpoint,
		Text:      a.Endpoint,
		Children:  []JsTreeDataModel{},
	}

	model.Type = a.Method
	model.Text = fmt.Sprintf(`<span class="%v">%v</span> %v`, a.getLabelByMethod(), a.Method, a.Endpoint)

	return model
}

const (
	// Version is the version x.y.z.
	Version = "1.0.5"
	// UpdateTime is the update time.
	UpdateTime = "2020-12-10 20:20:57"
)

type versionT struct {
	giu.T `url:"GET /version"`
}

// Version returns version information.
func (ctrl WebCliController) Version(_ versionT) gin.H {
	return gin.H{"version": Version, "updateTime": UpdateTime}
}

type treeT struct {
	giu.T `url:"GET /api/tree"`
}

// Tree return the api tree.
func (ctrl WebCliController) Tree(_ treeT) gin.H {
	apis := EndpointList(true)
	trees := make([]JsTreeDataModel, len(apis))

	for i, api := range apis {
		trees[i] = api.createJsTreeModel()
	}

	return gin.H{"id": "0", "key": "APIs", "text": "APIs", "state": gin.H{"opened": true}, "children": trees, "type": "root"}
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
	model, err := GetEndpoint(ID(c.Query("id")))
	if err != nil {
		return err
	}

	if model != nil {
		c.Header("Content-Disposition", `attachment; filename="`+model.Filename+`"`)
		c.Data(http.StatusOK, model.MimeType, model.FileContent)
	} else {
		c.Status(http.StatusNotFound)
	}

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

	mimeType = mime.TypeByExtension(path.Ext(file.Filename))
	filename = file.Filename
	f, _ := file.Open()
	fileContent, _ = ioutil.ReadAll(f)

	return mimeType, filename, fileContent
}

type deleteEndpointT struct {
	giu.T `url:"GET /api/deleteendpoint"`
}

// DeleteEndpoint ...
func (ctrl WebCliController) DeleteEndpoint(c *gin.Context, _ deleteEndpointT) {
	_ = DeleteEndpoint(c.Query("id"))

	c.JSON(http.StatusOK, gin.H{"success": "ok"})
}

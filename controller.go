package httplive

import (
	"github.com/bingoohuang/gor/giu"
	"github.com/bingoohuang/httplive/internal/process"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"mime"
	"net/http"
	"path"
	"path/filepath"
)

const (
	// Version is the version x.y.z.
	Version = "1.3.3"
	// UpdateTime is the update time.
	UpdateTime = "2021-08-03 11:40:24"
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
	trees := make([]process.JsTreeDataModel, len(apis))

	for i, api := range apis {
		trees[i] = api.CreateJsTreeModel()
	}

	return gin.H{"id": "0", "key": "APIs", "text": "APIs", "state": gin.H{"opened": true}, "children": trees, "type": "root"}
}

type backupT struct {
	giu.T `url:"GET /api/backup"`
}

// Backup ...
func (ctrl WebCliController) Backup(c *gin.Context, _ backupT) {
	_ = DBDo(func(dao *Dao) error {
		dao.Backup(c.Writer, filepath.Base(Environments.DBFile))
		return nil
	})
}

type downloadFileT struct {
	giu.T `url:"GET /api/downloadfile"`
}

// DownloadFile ...
func (ctrl WebCliController) DownloadFile(c *gin.Context, _ downloadFileT) error {
	model, err := GetEndpoint(process.ID(c.Query("id")))
	if err != nil {
		return err
	}

	if model != nil {
		c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": model.Filename}))
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
	model, err := GetEndpoint(process.ID(id))
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
func (ctrl WebCliController) Save(model process.APIDataModel, _ saveT) (giu.HTTPStatus, interface{}) {
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
func (ctrl WebCliController) SaveEndpoint(model process.APIDataModel, c *gin.Context, _ saveEndpointT) {
	mimeType, filename, fileContent := parseFileContent(c)
	if filename != "" {
		model.MimeType = mimeType
		model.Filename = filename
		model.FileContent = fileContent
	}

	if dp, err := SaveEndpoint(model); err != nil {
		c.PureJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	} else {
		c.PureJSON(http.StatusOK, gin.H{"data": dp})
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

	c.PureJSON(http.StatusOK, gin.H{"success": "ok"})
}

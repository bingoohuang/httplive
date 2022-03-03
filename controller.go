package httplive

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin/binding"

	"github.com/bingoohuang/gg/pkg/v"
	"github.com/bingoohuang/gor/giu"
	"github.com/bingoohuang/httplive/internal/process"
	"github.com/gin-gonic/gin"
)

type versionT struct {
	giu.T `url:"GET /version"`
}

// Version returns version information.
func (ctrl WebCliController) Version(_ versionT) gin.H {
	return gin.H{"version": v.AppVersion, "build": v.BuildTime, "go": v.GoVersion, "git": v.GitCommit}
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
		dao.Backup(c.Writer, path.Base(Envs.DBFile))
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

func decodeJSON(r io.Reader, obj interface{}) error {
	decoder := json.NewDecoder(r)
	if binding.EnableDecoderUseNumber {
		decoder.UseNumber()
	}
	if binding.EnableDecoderDisallowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(obj); err != nil {
		return err
	}
	return validate(obj)
}

func validate(obj interface{}) error {
	if binding.Validator == nil {
		return nil
	}
	return binding.Validator.ValidateStruct(obj)
}

// Save 保存body.
func (ctrl WebCliController) Save(c *gin.Context, _ saveT) (giu.HTTPStatus, interface{}) {
	endpoint := c.Query("endpoint")
	method := c.Query("method")
	body := c.Query("body")
	if body == "" {
		v, _ := ioutil.ReadAll(c.Request.Body)
		body = string(v)
	}
	var model process.APIDataModel
	if endpoint == "" {
		if err := decodeJSON(strings.NewReader(body), &model); err != nil {
			return giu.HTTPStatus(http.StatusBadRequest), gin.H{"error": err.Error()}
		}
	} else {
		model.Endpoint = endpoint
		if method == "" {
			method = "ANY"
		}
		model.Method = method
		model.Body = body
	}

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

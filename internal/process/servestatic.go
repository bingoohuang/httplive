package process

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/bingoohuang/gg/pkg/iox"
	"github.com/bingoohuang/gg/pkg/uid"
	"github.com/bingoohuang/httplive/pkg/shapeio"

	"github.com/bingoohuang/gg/pkg/man"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/gin-gonic/gin"
)

const (
	HlServerStatic = "serverStatic"
)

func init() {
	registerHlHandlers(HlServerStatic, func() HlHandler { return &ServeStatic{} })
}

type AllowMethods struct {
	Methods []string `json:"allowMethods"`
}

type MethodsAllowed interface {
	AllowMethods(method string) bool
}

func (a *AllowMethods) AllowMethods(method string) bool {
	return ss.AnyOfFold(method, a.Methods...)
}

type ServeStatic struct {
	Root  string `json:"root"`
	Dir   string `json:"dir"` // (empty) / list / grid
	Index string `json:"index"`

	DownloadRateLimit string `json:"downloadRateLimit"` // rate limit per second for downloading, empty for no limit

	Upload          bool   `json:"upload"`          // allow upload or not
	UploadMaxSize   string `json:"uploadMaxSize"`   // max size like 10M to allow uploading, empty for no limit
	UploadRateLimit string `json:"uploadRateLimit"` // rate limit per second for uploading, empty for no limit
	UploadMaxMemory string `json:"uploadMaxMemory"` // max memory usage for uploading, default 16MiB

	AllowMethods `json:"allowMethods"`

	dirFirst          bool
	downloadRateLimit uint64
	uploadMaxSize     uint64
	uploadRateLimit   uint64
	uploadMaxMemory   uint64
}

func (s *ServeStatic) AfterUnmashal() {
	var err error
	s.downloadRateLimit, err = man.ParseBytes(s.DownloadRateLimit)
	if err != nil {
		log.Printf("parse downloadRateLimit %s failed: %v", s.DownloadRateLimit, err)
	}

	s.uploadMaxSize, err = man.ParseBytes(s.UploadMaxSize)
	if err != nil {
		log.Printf("parse uploadMaxSize %s failed: %v", s.UploadMaxSize, err)
	}

	s.uploadRateLimit, err = man.ParseBytes(s.UploadRateLimit)
	if err != nil {
		log.Printf("parse uploadRateLimit %s failed: %v", s.UploadRateLimit, err)
	}

	s.uploadMaxMemory, err = man.ParseBytes(s.UploadMaxMemory)
	if err != nil {
		log.Printf("parse uploadMaxMemory %s failed: %v", s.UploadMaxMemory, err)
	}
	if s.uploadMaxMemory <= 0 {
		s.uploadMaxMemory = 16 /*16 MiB */ << 20
	}
}

type AfterUnmashaler interface {
	AfterUnmashal()
}

var (
	DirListTemplate *template.Template
	GridTemplate    *template.Template
)

func (s *ServeStatic) HlHandle(c *gin.Context, apiModel *APIDataModel, asset func(name string) string) error {
	rootStat, err := os.Stat(s.Root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.Status(http.StatusNotFound)
			return nil
		}

		return fmt.Errorf("root directory: %w", err)
	}

	if c.Request.Method == http.MethodGet && c.Query("dir") == "upload" || c.Request.Method == http.MethodPost {
		return s.serveUpload(c, apiModel, rootStat, asset)
	} else if c.Request.Method == http.MethodGet {
		return s.serveGet(c, apiModel, rootStat)
	}
	c.Status(http.StatusMethodNotAllowed)
	return nil
}

func (s *ServeStatic) serveUpload(c *gin.Context, apiModel *APIDataModel, rootStat os.FileInfo, asset func(name string) string) error {
	if !s.Upload || !rootStat.IsDir() {
		c.Status(http.StatusForbidden)
		return nil
	}

	r := c.Request
	if s.uploadMaxSize > 0 {
		r.Body = http.MaxBytesReader(c.Writer, r.Body, int64(s.uploadMaxSize))
	}

	if s.uploadRateLimit > 0 {
		limit := shapeio.WithRateLimit(float64(s.uploadRateLimit))
		r.Body = shapeio.NewReader(r.Body, limit)
		c.Writer = &limitResponseWriter{
			ResponseWriter: c.Writer,
			RateLimiter:    shapeio.NewRateLimiter(limit),
		}
	}

	start := time.Now()

	if err := r.ParseMultipartForm(int64(s.uploadMaxMemory)); err != nil {
		if err != http.ErrNotMultipart {
			return err
		}
	}

	if r.MultipartForm == nil || len(r.MultipartForm.File) == 0 {
		c.Header("Content-Type", "text/html; charset=utf-8")
		_, _ = c.Writer.Write([]byte(asset("upload.html")))
		return nil
	}

	totalSize := int64(0)
	var files []string
	var fileSizes []string
	for k, v := range r.MultipartForm.File {
		file, n, err := s.saveFormFile(c, apiModel, v[0])
		if err != nil {
			return err
		}
		totalSize += n
		files = append(files, file)
		fileSizes = append(fileSizes, man.Bytes(uint64(n)))
		log.Printf("recieved file %s: %s", k, file)
	}

	end := time.Now()
	c.JSON(http.StatusOK, UploadResult{
		Start:         start.UTC().Format(http.TimeFormat),
		End:           end.UTC().Format(http.TimeFormat),
		Files:         files,
		FileSizes:     fileSizes,
		MaxTempMemory: man.Bytes(s.uploadMaxMemory),
		LimitSize:     man.Bytes(s.uploadMaxSize),
		TotalSize:     man.Bytes(uint64(totalSize)),
		Cost:          end.Sub(start).String(),
	})
	return nil
}

type limitResponseWriter struct {
	gin.ResponseWriter
	*shapeio.RateLimiter
}

// Write writes bytes from p.
func (s *limitResponseWriter) Write(p []byte) (int, error) {
	n, err := s.ResponseWriter.Write(p)
	if err != nil || s.Limiter == nil {
		return n, err
	}

	err = s.WaitN(s.Context, n)
	return n, err
}

// UploadResult is the structure of download result.
type UploadResult struct {
	Files         []string
	FileSizes     []string
	TotalSize     string
	Cost          string
	Start         string
	End           string
	MaxTempMemory string
	LimitSize     string
}

func (s *ServeStatic) saveFormFile(c *gin.Context, apiModel *APIDataModel, fh *multipart.FileHeader) (string, int64, error) {
	file, err := fh.Open()
	if err != nil {
		return "", 0, err
	}

	filename := firstFilename(filepath.Base(fh.Filename), uid.New().String())
	fullPath := filepath.Join(s.DirPath(c, apiModel), filename)

	// use temporary file directly
	if f, ok := file.(*os.File); ok {
		n, err := file.Seek(0, io.SeekEnd)
		if err != nil {
			return "", n, err
		}
		if err := file.Close(); err != nil {
			return "", 0, err
		}
		if err := os.Rename(f.Name(), fullPath); err != nil {
			return "", 0, err
		}
		return fullPath, n, nil
	}

	n, err := writeChunk(fullPath, file)
	if err := file.Close(); err != nil {
		return "", 0, err
	}
	return fullPath, n, err
}

func openChunk(fullPath string) (f *os.File, err error) {
	f, err = os.OpenFile(fullPath, os.O_CREATE|os.O_RDWR, 0o755)
	if err != nil {
		return f, fmt.Errorf("open file %s error: %w", fullPath, err)
	}
	defer func() {
		if err != nil && f != nil {
			iox.Close(f)
			f = nil
		}
	}()

	return f, nil
}

func writeChunk(fullPath string, chunk io.Reader) (int64, error) {
	f, err := openChunk(fullPath)
	if err != nil {
		return 0, err
	}

	defer iox.Close(f)

	n, err := io.Copy(f, chunk)
	if err != nil {
		return 0, fmt.Errorf("write file %s error: %w", fullPath, err)
	}

	return n, nil
}

func firstFilename(s ...string) string {
	for _, i := range s {
		if i != "" && i != "/" {
			return i
		}
	}

	return ""
}

// TrimExt trim ext from the right of filepath.
func TrimExt(filepath, ext string) string {
	return filepath[:len(filepath)-len(ext)]
}

func (s *ServeStatic) serveGet(c *gin.Context, apiModel *APIDataModel, rootStat os.FileInfo) error {
	if !rootStat.IsDir() { // not a directory
		c.File(s.Root)
		return nil
	}

	dirPath := s.DirPath(c, apiModel)
	fstat, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.Status(http.StatusNotFound)
		}
		log.Printf("stat %s failed: %v", dirPath, err)
		return nil
	}

	if !fstat.IsDir() {
		c.File(dirPath)
		return nil
	}

	if s.Index != "" {
		indexFile := path.Join(dirPath, s.Index)
		if indexFileStat, err := os.Stat(indexFile); err != nil || indexFileStat.IsDir() {
			s.Index = ""
		}
	}

	if dir := c.Query("dir"); dir != "" {
		s.dirFirst = true
		s.Dir = dir
	}

	if !s.dirFirst && s.Index != "" {
		c.File(path.Join(dirPath, s.Index))
		return nil
	}

	switch s.Dir {
	case "grid":
		return s.listPage(c, dirPath)
	case "list":
		return s.listPage(c, dirPath)
	}

	if s.Index != "" {
		c.File(path.Join(dirPath, s.Index))
	} else {
		s.tryIndexHtml(c)
	}

	return nil
}

func (s *ServeStatic) DirPath(c *gin.Context, apiModel *APIDataModel) string {
	urlPath := c.Request.URL.Path
	fixPath, _ := ParsePathParams(apiModel)
	urlPath = strings.TrimPrefix(urlPath, fixPath)
	dirPath := path.Join(s.Root, urlPath)
	return dirPath
}

func (s ServeStatic) tryIndexHtml(c *gin.Context) {
	f := path.Join(s.Root, "index.html")
	if stat, err := os.Stat(f); err == nil && !stat.IsDir() {
		c.File(f)
		return
	}
	f = path.Join(s.Root, "index.htm")
	if stat, err := os.Stat(f); err == nil && !stat.IsDir() {
		c.File(f)
		return
	}

	c.Status(http.StatusNotFound)
}

func (s ServeStatic) listPage(c *gin.Context, dir string) error {
	data, err := ListDir(dir, c.Request.URL.RawQuery, 1000)
	if err != nil {
		return err
	}
	c.Header("Content-Type", "text/html; charset=utf-8")

	if s.Dir == "grid" {
		var imageFiles []File
		for _, d := range data.Files {
			name := strings.ToLower(d.Name)
			if ss.HasSuffix(name, ".jpg", ".jpeg", ".png") {
				imageFiles = append(imageFiles, d)
			}
		}

		if len(imageFiles) > 0 {
			data.Files = imageFiles
			return GridTemplate.Execute(c.Writer, data)
		}

	}
	return DirListTemplate.Execute(c.Writer, data)
}

func ParsePathParams(apiModel *APIDataModel) (prefix string, hasParams bool) {
	segments := strings.Split(apiModel.Endpoint, "/")
	for i, seg := range segments {
		if ss.HasPrefix(seg, "*", ":") {
			return strings.Join(segments[:i], "/"), true
		}
	}
	prefix = strings.Join(segments, "/")
	return apiModel.Endpoint, false
}

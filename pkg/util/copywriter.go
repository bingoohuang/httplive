package util

import (
	"bytes"
	"strings"

	"github.com/bingoohuang/golog/pkg/hlog"

	"github.com/gin-gonic/gin"
)

type GinCopyWriter struct {
	gin.ResponseWriter
	Buf bytes.Buffer
	c   *gin.Context
}

// NewGinCopyWriter creates a new GinCopyWriter.
func NewGinCopyWriter(w gin.ResponseWriter, c *gin.Context) *GinCopyWriter {
	return &GinCopyWriter{ResponseWriter: w, c: c}
}

func (w *GinCopyWriter) Write(data []byte) (n int, err error) {
	if ct := w.c.GetHeader("Content-Type"); strings.Contains(ct, "json") {
		w.Buf.Write(data)
	}

	return w.ResponseWriter.Write(data)
}

func (w *GinCopyWriter) WriteString(s string) (n int, err error) {
	w.Buf.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

func (w *GinCopyWriter) Body(maxSize int) string {
	if w.ResponseWriter.Size() <= maxSize {
		return w.Buf.String()
	}

	payload, extra := hlog.AbbreviateBytes(w.Buf.Bytes(), maxSize)
	return payload + extra
}

func (w *GinCopyWriter) Bytes() []byte {
	return w.Buf.Bytes()
}

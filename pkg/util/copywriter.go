package util

import (
	"bytes"

	"github.com/gin-gonic/gin"
)

type GinCopyWriter struct {
	gin.ResponseWriter
	Buf bytes.Buffer
}

// NewGinCopyWriter creates a new GinCopyWriter.
func NewGinCopyWriter(w gin.ResponseWriter) *GinCopyWriter {
	return &GinCopyWriter{ResponseWriter: w}
}

func (w *GinCopyWriter) Write(data []byte) (n int, err error) {
	w.Buf.Write(data)
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

	return string(w.Buf.Bytes()[:maxSize-3]) + "..."
}

func (w *GinCopyWriter) Bytes() []byte {
	return w.Buf.Bytes()
}

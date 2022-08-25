package httptee

import (
	"bytes"
	"io"
	"net/http"
)

// DuplicateRequest duplicate http req
func DuplicateRequest(request *http.Request) *http.Request {
	var bodyBytes []byte
	if request.Body != nil {
		bodyBytes, _ = io.ReadAll(request.Body)
	}

	request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	return &http.Request{
		Method:        request.Method,
		URL:           CloneURL(request.URL),
		Proto:         request.Proto,
		ProtoMajor:    request.ProtoMajor,
		ProtoMinor:    request.ProtoMinor,
		Header:        request.Header,
		Body:          io.NopCloser(bytes.NewBuffer(bodyBytes)),
		Host:          request.Host,
		ContentLength: request.ContentLength,
	}
}

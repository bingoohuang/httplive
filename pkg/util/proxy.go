package util

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

// ReverseProxy reverse proxy originalPath to targetHost with targetPath.
// And the relative forwarding is rewritten.
func ReverseProxy(originalPath, targetHost, targetPath string, timeout time.Duration) *httputil.ReverseProxy {
	director := func(req *http.Request) {
		req.URL.Scheme = "http"

		req.URL.Host = targetHost
		req.URL.Path = targetPath

		req.Header.Add("X-Forwarded-Host", req.Host)
		req.Header.Add("X-Origin-Host", req.Header.Get("Host"))
	}

	modifyResponse := func(r *http.Response) error {
		respLocationHeader := r.Header.Get("Location")
		if IsRelativeForward(r.StatusCode, respLocationHeader) {
			// 301/302时，本地相对路径跳转时，改写Location返回头
			basePath := strings.TrimRight(originalPath, targetPath)
			r.Header.Set("Location", basePath+respLocationHeader)
		}
		r.Header.Set("Proxied", "Httplive")

		return nil
	}
	transport := &http.Transport{DialContext: TimeoutDialer(timeout, timeout)}

	// 更多可以参见 https://github.com/Integralist/go-reverse-proxy/blob/master/proxy/proxy.go
	return &httputil.ReverseProxy{Director: director, ModifyResponse: modifyResponse, Transport: transport}
}

// Dialer defines dialer function alias
type Dialer func(ctx context.Context, net, addr string) (c net.Conn, err error)

// TimeoutDialer returns functions of connection dialer with timeout settings for http.Transport Dial field.
// https://gist.github.com/c4milo/275abc6eccbfd88ad56ca7c77947883a
// HTTP client with support for read and write timeouts which are missing in Go's standard library.
func TimeoutDialer(cTimeout time.Duration, rwTimeout time.Duration) Dialer {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := net.DialTimeout(network, addr, cTimeout)
		if err != nil {
			return conn, err
		}

		if rwTimeout > 0 {
			err = conn.SetDeadline(time.Now().Add(rwTimeout))
		}

		return conn, err
	}
}

// IsRelativeForward tells the statusCode is 301/302 and locationHeader is relative
func IsRelativeForward(statusCode int, locationHeader string) bool {
	switch statusCode {
	case http.StatusMovedPermanently, http.StatusFound:
	default:
		return false
	}

	return !HasPrefix(locationHeader, "http://", "https://")
}

// AnyOf tells s equals any element of p...
func AnyOf(s string, p ...string) bool {
	for _, i := range p {
		if s == i {
			return true
		}
	}

	return false
}

// HasPrefix tells s has any prefix of p...
func HasPrefix(s string, p ...string) bool {
	for _, i := range p {
		if strings.HasPrefix(s, i) {
			return true
		}
	}

	return false
}

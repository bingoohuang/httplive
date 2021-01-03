package httptee

import (
	"crypto/tls"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CloneURL clones a URL.
func CloneURL(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}

	u2 := new(url.URL)
	*u2 = *u

	if u.User != nil {
		u2.User = new(url.Userinfo)
		*u2.User = *u.User
	}

	return u2
}

// SetRequestTarget sets the req URL.
// this turns a inbound req (a req without URL) into an outbound req.
func SetRequestTarget(request *http.Request, b Backend) {
	request.URL.Scheme = b.Addr.Scheme
	request.URL.Host = b.Host
	request.URL.Path = b.Addr.Path
}

func CreateHandler(addrs string) (*Handler, error) {
	var backends []Backend

	for _, tok := range strings.Split(addrs, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}

		b := Backend{}

		if err := b.ParseAddress(tok); err != nil {
			log.Printf("E! failed to parse %s, error", tok)
			continue
		}

		backends = append(backends, b)
	}

	h := &Handler{
		Alternatives: backends,
	}
	if len(h.Alternatives) > 0 {
		h.workers = NewWorkerPool(20)
	}

	return h, nil
}

// MakeTransport makes a new http.Transport.
func MakeTransport(t time.Duration, closeConnections bool) *http.Transport {
	return &http.Transport{
		DialContext:           (&net.Dialer{Timeout: t, KeepAlive: 10 * t}).DialContext,
		DisableKeepAlives:     closeConnections,
		TLSHandshakeTimeout:   t,
		ResponseHeaderTimeout: t,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, // nolint
	}
}

// handleAlterRequest duplicate req and sent it to alternative Backend
func (h *Handler) handleAlterRequest(r AlternativeReq, t http.RoundTripper) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in ServeHTTP(alternate req) from:", r)
		}
	}()

	if rsp := handleRequest(r.req, t); rsp != nil {
		_, _ = io.Copy(ioutil.Discard, rsp.Body)
		_ = rsp.Body.Close()
	}
}

// handleRequest sends a req and returns the response.
func handleRequest(request *http.Request, t http.RoundTripper) (rsp *http.Response) {
	var err error

	if rsp, err = t.RoundTrip(request); err != nil {
		log.Println("Request failed:", err)
	}

	return
}

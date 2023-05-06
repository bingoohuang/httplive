package httptee

import (
	"net/http"
	"net/url"

	"github.com/bingoohuang/httplive/pkg/util"
)

// Handler contains the address of the main PrimaryTarget and the one for the Host target
type Handler struct {
	workers      Pool
	Alternatives []Backend
}

// Backend represents the backend server.
type Backend struct {
	Addr *url.URL
	Host string
}

// AlternativeReq represents the alternative request.
type AlternativeReq struct {
	req     *http.Request
	Handler *Handler
}

// Run Do do the request.
func (r AlternativeReq) Run() error {
	r.Handler.handleAlterRequest(r, util.Transport)
	return nil
}

// ParseAddress parses an address to https, host(ip:port)
func (b *Backend) ParseAddress(addr string) (err error) {
	if b.Addr, err = url.Parse(addr); err != nil {
		return err
	}

	if b.Addr.Scheme == "" {
		b.Addr.Scheme = "http"
	}

	https := b.Addr.Scheme == "https"
	b.Host = b.Addr.Host

	if b.Addr.Port() == "" {
		if https {
			b.Host += ":443"
		} else {
			b.Host += ":80"
		}
	}

	return nil
}

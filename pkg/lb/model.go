package lb

import (
	"net/url"
	"sync"
)

// BackendPool holds information about reachable backends
type BackendPool struct {
	backends []*Backend
	total    uint64
	current  uint64
}

// Backend holds the data about a server
type Backend struct {
	alive bool
	Host  string // ip:port
	mux   sync.RWMutex
	Addr  *url.URL
}

// SetAlive for this backend
func (b *Backend) SetAlive(alive bool) {
	b.mux.Lock()
	b.alive = alive
	b.mux.Unlock()
}

// Alive returns true when backend is alive
func (b *Backend) Alive() (alive bool) {
	b.mux.RLock()
	alive = b.alive
	b.mux.RUnlock()

	return
}

// ParseAddress parses an address to https, host(ip:port)
func (b *Backend) ParseAddress(addr string) (err error) {
	if b.Addr, err = url.Parse(addr); err != nil {
		return err
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

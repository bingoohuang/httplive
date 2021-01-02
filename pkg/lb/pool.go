package lb

import (
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

// CreateServerPool creates a server pool by serverList
func CreateServerPool(serverList string) *BackendPool {
	var serverPool BackendPool

	for _, tok := range strings.Split(serverList, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}

		b := &Backend{alive: true}

		if err := b.ParseAddress(tok); err != nil {
			log.Printf("E! failed to parse %s, error", tok)
			continue
		}

		serverPool.Add(b)

		log.Printf("Configured server: %s", tok)
	}

	return &serverPool
}

// CheckBackends check backends
func (s *BackendPool) CheckBackends() error {
	if s.total == 0 {
		return errors.New("Please provide one or more backends to load balance")
	}

	return nil
}

// Add adds a backend to the server pool
func (s *BackendPool) Add(backend *Backend) {
	s.backends = append(s.backends, backend)
	s.total++
}

// nextIndex atomically increase the counter and return an index
func (s *BackendPool) nextIndex() uint64 {
	return atomic.AddUint64(&s.current, 1) % s.total
}

// GetNextPeer returns next active peer to take a connection
func (s *BackendPool) GetNextPeer() *Backend {
	if s.total == 1 {
		return s.backends[0]
	}

	next := s.nextIndex() // loop entire backends to find out an alive backend

	for i := next; i < next+s.total; i++ {
		idx := i % s.total

		if !s.backends[idx].Alive() {
			continue
		}

		// if we have an alive backend, use it and store if its not the original one
		if i != next {
			atomic.StoreUint64(&s.current, idx)
		}

		return s.backends[idx]
	}

	// 如果全部下线，则选择第一个进行尝试
	return s.backends[next]
}

// healthCheck pings the backends and update the status
func (s *BackendPool) healthCheck() {
	for _, b := range s.backends {
		oldAlive := b.Alive()
		alive := IsAddressAlive(b.Host)

		if oldAlive == alive {
			continue
		}

		b.SetAlive(alive)

		log.Printf("HealthCheck %s alive=%v\n", b.Host, alive)
	}
}

// HealthCheck runs a routine for check status of the backends every 20s
func (s *BackendPool) HealthCheck() {
	s.healthCheck()

	for range time.NewTicker(time.Second * 20).C {
		s.healthCheck()
	}
}

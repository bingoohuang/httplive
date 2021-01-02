package lb

import (
	"net"
	"net/http"
	"time"
)

type key int

const (
	// Attempts represents the key of attempts times.
	Attempts key = iota
	// Retries represents the key of retry times.
	Retries
)

// GetAttempts returns the attempts for request
func GetAttempts(r *http.Request) int {
	if attempts, ok := r.Context().Value(Attempts).(int); ok {
		return attempts
	}

	return 1
}

// GetRetry returns the retries for request
func GetRetry(r *http.Request) int {
	if retry, ok := r.Context().Value(Retries).(int); ok {
		return retry
	}

	return 0
}

// IsAddressAlive checks whether an address is alive by establishing a TCP connection
func IsAddressAlive(address string) bool {
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return false
	}

	_ = conn.Close()
	return true
}

package shapeio

import (
	"context"
	"io"
	"time"

	"golang.org/x/time/rate"
)

const burstLimit = 1000 * 1000 * 1000

type RateLimiterSetter interface {
	SetContext(ctx context.Context)
	SetRateLimit(bytesPerSec float64)
}

type RateLimiter struct {
	*rate.Limiter
	context.Context
}

func (s *RateLimiter) SetContext(ctx context.Context) {
	s.Context = ctx
}

// SetRateLimit sets rate limit (bytes/sec) to the reader.
func (s *RateLimiter) SetRateLimit(bytesPerSec float64) {
	s.Limiter = rate.NewLimiter(rate.Limit(bytesPerSec), burstLimit)
	s.Limiter.AllowN(time.Now(), burstLimit) // spend initial burst
}

type Reader struct {
	io.ReadCloser
	RateLimiter
}

type Writer struct {
	io.WriteCloser
	RateLimiter
}

type LimitConfig struct {
	context.Context
	RateLimit float64
}

type LimitConfigFn func(*LimitConfig)

func WithContext(ctx context.Context) LimitConfigFn {
	return func(c *LimitConfig) {
		c.Context = ctx
	}
}

func WithRateLimit(rateLimit float64) LimitConfigFn {
	return func(c *LimitConfig) {
		c.RateLimit = rateLimit
	}
}

// NewReader returns a reader that implements io.Reader with rate limiting.
func NewReader(r io.Reader, fns ...LimitConfigFn) *Reader {
	s := &Reader{ReadCloser: WrapReadCloser(r)}
	NewRateLimiterSetter(s, fns...)
	return s
}

// NewWriter returns a writer that implements io.Writer with rate limiting.
func NewWriter(w io.Writer, fns ...LimitConfigFn) *Writer {
	s := &Writer{WriteCloser: WrapWriteCloser(w)}
	NewRateLimiterSetter(s, fns...)
	return s
}

func NewRateLimiter(fns ...LimitConfigFn) *RateLimiter {
	l := &RateLimiter{}
	NewRateLimiterSetter(l, fns...)
	return l
}

func NewRateLimiterSetter(w RateLimiterSetter, fns ...LimitConfigFn) {
	c := &LimitConfig{}
	for _, fn := range fns {
		fn(c)
	}
	if c.Context == nil {
		c.Context = context.Background()
	}

	w.SetContext(c.Context)

	if c.RateLimit > 0 {
		w.SetRateLimit(c.RateLimit)
	}
}

// Read reads bytes into p.
func (s *Reader) Read(p []byte) (int, error) {
	n, err := s.ReadCloser.Read(p)
	if err != nil || s.Limiter == nil {
		return n, err
	}

	err = s.WaitN(s.Context, n)
	return n, err
}

// Write writes bytes from p.
func (s *Writer) Write(p []byte) (int, error) {
	n, err := s.WriteCloser.Write(p)
	if err != nil || s.Limiter == nil {
		return n, err
	}

	err = s.WaitN(s.Context, n)
	return n, err
}

// WrapReadCloser returns a ReadCloser with Close method wrapping the provided Reader r.
func WrapReadCloser(r io.Reader) io.ReadCloser {
	return wrapReadCloser{r}
}

type wrapReadCloser struct {
	io.Reader
}

func (n wrapReadCloser) Close() error {
	if c, ok := n.Reader.(io.Closer); ok {
		return c.Close()
	}

	return nil
}

// WrapWriteCloser returns a WriteCloser with Close method wrapping the provided Writer r.
func WrapWriteCloser(r io.Writer) io.WriteCloser {
	return wrapWriteCloser{r}
}

type wrapWriteCloser struct {
	io.Writer
}

func (n wrapWriteCloser) Close() error {
	if c, ok := n.Writer.(io.Closer); ok {
		return c.Close()
	}

	return nil
}

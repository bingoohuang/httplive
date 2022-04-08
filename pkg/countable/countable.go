package countable

import (
	"sync"
	"sync/atomic"
)

// Counter Stores counts associated with a key.
type Counter struct {
	sync.Map
}

// GetValue Retrieves the count without modifying it
func (c *Counter) GetValue(key string) int64 {
	if count, ok := c.Load(key); ok {
		return atomic.LoadInt64(count.(*int64))
	}
	return 0
}

func (c *Counter) Range(f func(key string, value int64) bool) {
	c.Map.Range(func(key, value any) bool {
		return f(key.(string), *value.(*int64))
	})
}

// Get Retrieves the count without modifying it
func (c *Counter) Get(key string) (int64, bool) {
	if count, ok := c.Load(key); ok {
		return atomic.LoadInt64(count.(*int64)), true
	}
	return 0, false
}

// Add Adds value to the stored underlying value if it exists.
// If it does not exist, the value is assigned to the key.
func (c *Counter) Add(key string, value int64) int64 {
	count, loaded := c.LoadOrStore(key, &value)
	if loaded {
		return atomic.AddInt64(count.(*int64), value)
	}
	return *count.(*int64)
}

// DeleteAndGetLastValue Deletes the value associated with the key and retrieves it.
func (c *Counter) DeleteAndGetLastValue(key string) (lastValue int64, loaded bool) {
	last, loaded := c.LoadAndDelete(key)
	if loaded {
		return *last.(*int64), loaded
	}
	return 0, false
}

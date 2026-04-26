package dialect

import "sync"

type Dialect interface {
	Name() string
	Placeholder(n int) string
	QuoteIdent(ident string) string
	SupportsReturning() bool
}

var (
	mu       sync.RWMutex
	dialects = map[string]Dialect{}
)

func Register(driverName string, d Dialect) {
	mu.Lock()
	defer mu.Unlock()
	dialects[driverName] = d
}

func Get(driverName string) (Dialect, bool) {
	mu.RLock()
	defer mu.RUnlock()
	d, ok := dialects[driverName]
	return d, ok
}

func MustGet(driverName string) Dialect {
	d, ok := Get(driverName)
	if !ok || d == nil {
		panic("corm: unsupported dialect: " + driverName)
	}
	return d
}

const maxQuoteCacheSize = 2048

type quoteCache struct {
	mu    sync.RWMutex
	items map[string]string
	count int
}

func (c *quoteCache) Get(key string) (string, bool) {
	c.mu.RLock()
	v, ok := c.items[key]
	c.mu.RUnlock()
	return v, ok
}

func (c *quoteCache) Set(key, value string) {
	c.mu.Lock()
	if c.items == nil {
		c.items = make(map[string]string, 256)
	}
	if c.count < maxQuoteCacheSize {
		if _, ok := c.items[key]; !ok {
			c.items[key] = value
			c.count++
		}
	}
	c.mu.Unlock()
}

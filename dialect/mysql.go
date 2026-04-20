package dialect

import (
	"strings"
	"sync"
)

const maxQuoteCacheSize = 2048

type mysqlDialect struct {
	mu       sync.RWMutex
	cache    map[string]string
	cacheLen int
}

func (d *mysqlDialect) Name() string { return "mysql" }

func (d *mysqlDialect) Placeholder(n int) string { return "?" }

func (d *mysqlDialect) QuoteIdent(ident string) string {
	if ident == "" {
		return "``"
	}

	// Fast path: no backticks in ident
	if strings.IndexByte(ident, '`') == -1 {
		result := "`" + ident + "`"
		if len(ident) <= 64 {
			d.mu.RLock()
			if v, ok := d.cache[ident]; ok {
				d.mu.RUnlock()
				return v
			}
			d.mu.RUnlock()

			d.mu.Lock()
			if d.cache == nil {
				d.cache = make(map[string]string, 256)
			}
			if d.cacheLen < maxQuoteCacheSize {
				if _, ok := d.cache[ident]; !ok {
					d.cache[ident] = result
					d.cacheLen++
				}
			}
			d.mu.Unlock()
		}
		return result
	}

	// Escape backticks - inline replacement to avoid strings.ReplaceAll allocation
	var result strings.Builder
	result.Grow(len(ident) + 2)
	result.WriteByte('`')
	for i := 0; i < len(ident); i++ {
		c := ident[i]
		if c == '`' {
			result.WriteString("``")
		} else {
			result.WriteByte(c)
		}
	}
	result.WriteByte('`')
	return result.String()
}

func (d *mysqlDialect) SupportsReturning() bool { return false }

func init() {
	Register("mysql", &mysqlDialect{})
}

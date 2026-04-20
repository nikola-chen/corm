package dialect

import (
	"strconv"
	"strings"
	"sync"
)

var postgresPlaceholders = [...]string{
	"$1", "$2", "$3", "$4", "$5", "$6", "$7", "$8", "$9", "$10",
	"$11", "$12", "$13", "$14", "$15", "$16", "$17", "$18", "$19", "$20",
}

const maxPgQuoteCacheSize = 2048

type postgresDialect struct {
	mu       sync.RWMutex
	cache    map[string]string
	cacheLen int
}

func (d *postgresDialect) Name() string { return "postgres" }

func (d *postgresDialect) Placeholder(n int) string {
	if n > 0 && n <= 20 {
		return postgresPlaceholders[n-1]
	}
	return "$" + strconv.Itoa(n)
}

func (d *postgresDialect) QuoteIdent(ident string) string {
	if ident == "" {
		return `""`
	}

	// Fast path: no double quotes in ident
	if strings.IndexByte(ident, '"') == -1 {
		result := `"` + ident + `"`
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
			if d.cacheLen < maxPgQuoteCacheSize {
				if _, ok := d.cache[ident]; !ok {
					d.cache[ident] = result
					d.cacheLen++
				}
			}
			d.mu.Unlock()
		}
		return result
	}

	// Escape double quotes - inline replacement to avoid strings.ReplaceAll allocation
	var result strings.Builder
	result.Grow(len(ident) + 2)
	result.WriteByte('"')
	for i := 0; i < len(ident); i++ {
		c := ident[i]
		if c == '"' {
			result.WriteString(`""`)
		} else {
			result.WriteByte(c)
		}
	}
	result.WriteByte('"')
	return result.String()
}

func (d *postgresDialect) SupportsReturning() bool { return true }

func init() {
	Register("postgres", &postgresDialect{})
	Register("postgresql", &postgresDialect{})
}

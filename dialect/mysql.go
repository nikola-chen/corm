package dialect

import (
	"strings"
	"sync"
)

// quoteCache caches quoted identifiers to avoid repeated allocations.
// Key: ident string, Value: quoted string
var quoteCache sync.Map

// maxQuoteCacheSize limits the cache size to prevent unbounded growth.
const maxQuoteCacheSize = 2048

type mysqlDialect struct{}

func (d mysqlDialect) Name() string { return "mysql" }

func (d mysqlDialect) Placeholder(n int) string { return "?" }

func (d mysqlDialect) QuoteIdent(ident string) string {
	if ident == "" {
		return "``"
	}
	
	// Check cache first
	if cached, ok := quoteCache.Load(ident); ok {
		return cached.(string)
	}
	
	// Fast path: no backticks in ident
	if strings.IndexByte(ident, '`') == -1 {
		result := "`" + ident + "`"
		// Cache common identifiers (reasonable length)
		if len(ident) <= 64 {
			quoteCache.Store(ident, result)
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

func (d mysqlDialect) SupportsReturning() bool { return false }

func init() {
	Register("mysql", mysqlDialect{})
}

package dialect

import (
	"strings"
	"sync"
	"sync/atomic"
)

var quoteCache sync.Map
var quoteCacheCount atomic.Int64
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
		if len(ident) <= 64 {
			if quoteCacheCount.Load() < maxQuoteCacheSize {
				if _, loaded := quoteCache.LoadOrStore(ident, result); !loaded {
					quoteCacheCount.Add(1)
				}
			}
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

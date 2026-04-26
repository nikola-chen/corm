package dialect

import (
	"strconv"
	"strings"
)

var postgresPlaceholders = [...]string{
	"$1", "$2", "$3", "$4", "$5", "$6", "$7", "$8", "$9", "$10",
	"$11", "$12", "$13", "$14", "$15", "$16", "$17", "$18", "$19", "$20",
}

type postgresDialect struct {
	cache quoteCache
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

	if strings.IndexByte(ident, '"') == -1 {
		result := `"` + ident + `"`
		if len(ident) <= 64 {
			if v, ok := d.cache.Get(ident); ok {
				return v
			}
			d.cache.Set(ident, result)
		}
		return result
	}

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

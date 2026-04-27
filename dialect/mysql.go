package dialect

import "strings"

type mysqlDialect struct {
	cache quoteCache
}

func (d *mysqlDialect) Name() string { return "mysql" }

func (d *mysqlDialect) Placeholder(n int) string { return "?" }

func (d *mysqlDialect) QuoteIdent(ident string) string {
	if ident == "" {
		return "``"
	}

	if strings.IndexByte(ident, '`') == -1 {
		result := "`" + ident + "`"
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
	result.WriteByte('`')
	for i := range ident {
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

package dialect

import (
	"bytes"
	"strconv"
	"strings"
)

type postgresDialect struct{}

func (d postgresDialect) Name() string { return "postgres" }

func (d postgresDialect) Placeholder(n int) string { return "$" + strconv.Itoa(n) }

func (d postgresDialect) QuoteIdent(ident string) string {
	if ident == "" {
		return `""`
	}
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}

func (d postgresDialect) SupportsReturning() bool { return true }

func isDollarTag(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}

func (d postgresDialect) RewritePlaceholders(sql string, startIndex int) (string, int) {
	if strings.IndexByte(sql, '?') < 0 {
		return sql, startIndex
	}
	var b bytes.Buffer
	b.Grow(len(sql) + 8)

	idx := startIndex
	for i := 0; i < len(sql); {
		c := sql[i]

		if c == '-' && i+1 < len(sql) && sql[i+1] == '-' {
			b.WriteString(sql[i:])
			break
		}
		if c == '/' && i+1 < len(sql) && sql[i+1] == '*' {
			j := i + 2
			for j+1 < len(sql) && !(sql[j] == '*' && sql[j+1] == '/') {
				j++
			}
			if j+1 < len(sql) {
				j += 2
			}
			b.WriteString(sql[i:j])
			i = j
			continue
		}

		if c == '\'' {
			b.WriteByte('\'')
			i++
			for i < len(sql) {
				ch := sql[i]
				b.WriteByte(ch)
				i++
				if ch == '\'' {
					if i < len(sql) && sql[i] == '\'' {
						b.WriteByte('\'')
						i++
						continue
					}
					break
				}
			}
			continue
		}

		if c == '"' {
			b.WriteByte('"')
			i++
			for i < len(sql) {
				ch := sql[i]
				b.WriteByte(ch)
				i++
				if ch == '"' {
					if i < len(sql) && sql[i] == '"' {
						b.WriteByte('"')
						i++
						continue
					}
					break
				}
			}
			continue
		}

		if c == '$' {
			j := i + 1
			for j < len(sql) && sql[j] != '$' {
				j++
			}
			if j < len(sql) {
				tag := sql[i+1 : j]
				if isDollarTag(tag) {
					delim := sql[i : j+1]
					k := strings.Index(sql[j+1:], delim)
					if k >= 0 {
						end := j + 1 + k + len(delim)
						b.WriteString(sql[i:end])
						i = end
						continue
					}
				}
			}
		}

		if c == '?' {
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(idx))
			idx++
			i++
			continue
		}

		b.WriteByte(c)
		i++
	}

	return b.String(), idx
}

func init() {
	Register("postgres", postgresDialect{})
	Register("postgresql", postgresDialect{})
}

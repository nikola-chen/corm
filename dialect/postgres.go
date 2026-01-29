package dialect

import (
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

func init() {
	Register("postgres", postgresDialect{})
	Register("postgresql", postgresDialect{})
}

package dialect

import "strings"

type mysqlDialect struct{}

func (d mysqlDialect) Name() string { return "mysql" }

func (d mysqlDialect) Placeholder(n int) string { return "?" }

func (d mysqlDialect) QuoteIdent(ident string) string {
	if ident == "" {
		return "``"
	}
	return "`" + strings.ReplaceAll(ident, "`", "``") + "`"
}

func (d mysqlDialect) SupportsReturning() bool { return false }

func init() {
	Register("mysql", mysqlDialect{})
}

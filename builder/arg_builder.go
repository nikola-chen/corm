package builder

import (
	"fmt"
	"strings"

	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/dialect"
)

const (
	mysqlPlaceholder = "?"
	maxSQLLength     = 1024 * 1024
)

type argBuilder struct {
	d         dialect.Dialect
	idx       int
	args      []any
	usesQmark bool
	buf       *strings.Builder
}

func newArgBuilder(d dialect.Dialect, buf *strings.Builder) *argBuilder {
	return &argBuilder{
		d:         d,
		idx:       1,
		args:      make([]any, 0, 32),
		usesQmark: d.Placeholder(1) == "?",
		buf:       buf,
	}
}

func (a *argBuilder) usesQuestionPlaceholders() bool {
	return a.usesQmark
}

func (a *argBuilder) add(v any) string {
	a.args = append(a.args, v)
	if a.usesQmark {
		a.idx++
		return mysqlPlaceholder
	}
	p := a.d.Placeholder(a.idx)
	a.idx++
	return p
}

func (a *argBuilder) appendExpr(e clause.Expr) error {
	sql := e.SQL
	if len(sql) > 0 && sql[0] == ' ' {
		sql = sql[1:]
	}
	if len(sql) == 0 {
		return nil
	}
	if a.buf.Len()+len(sql) > maxSQLLength {
		return errSQLTooLong
	}
	if len(e.Args) == 0 {
		a.buf.WriteString(sql)
		return nil
	}
	if strings.IndexByte(sql, '?') < 0 {
		return errMissingPlaceholders
	}
	expected := len(e.Args)
	if a.usesQuestionPlaceholders() {
		if count := countQuestionPlaceholders(sql, a.d.Name() == "mysql"); count != expected {
			return fmt.Errorf("corm: placeholder count mismatch: expected %d, got %d", expected, count)
		}
		a.buf.WriteString(sql)
		a.args = append(a.args, e.Args...)
		a.idx += expected
		return nil
	}
	rewritten, next, err := rewritePlaceholders(sql, a.idx, a.d.Placeholder, a.d.Name() == "postgres", a.d.Name() == "mysql")
	if err != nil {
		return err
	}
	if next-a.idx != expected {
		return fmt.Errorf("corm: placeholder count mismatch: expected %d, got %d", expected, next-a.idx)
	}
	if err := a.checkAndWrite(rewritten); err != nil {
		return err
	}
	a.args = append(a.args, e.Args...)
	a.idx = next
	return nil
}



func (a *argBuilder) checkAndWrite(sql string) error {
	if a.buf.Len()+len(sql) > maxSQLLength {
		return errSQLTooLong
	}
	a.buf.WriteString(sql)
	return nil
}

type sqlTokenType int

const (
	sqlTokenText        sqlTokenType = iota
	sqlTokenPlaceholder              // ?
	sqlTokenSingleQuote
	sqlTokenDoubleQuote
	sqlTokenLineComment
	sqlTokenBlockComment
	sqlTokenDollarQuote
)

type sqlToken struct {
	kind sqlTokenType
	text string
	end  int
}

func tokenizeSQL(sql string, allowBackslashEscape bool) []sqlToken {
	var tokens []sqlToken
	n := len(sql)
	i := 0

	for i < n {
		switch {
		case i+1 < n && sql[i] == '-' && sql[i+1] == '-':
			start := i
			i += 2
			for i < n && sql[i] != '\n' {
				i++
			}
			if i < n {
				i++
			}
			tokens = append(tokens, sqlToken{kind: sqlTokenLineComment, text: sql[start:i], end: i})

		case i+1 < n && sql[i] == '/' && sql[i+1] == '*':
			start := i
			i += 2
			for i+1 < n && !(sql[i] == '*' && sql[i+1] == '/') {
				i++
			}
			if i+1 < n {
				i += 2
			} else {
				i = n
			}
			tokens = append(tokens, sqlToken{kind: sqlTokenBlockComment, text: sql[start:i], end: i})

		case sql[i] == '\'':
			start := i
			i++
			for i < n {
				if allowBackslashEscape && sql[i] == '\\' {
					i += 2
					if i > n {
						i = n
					}
					continue
				}
				if sql[i] == '\'' {
					i++
					if i < n && sql[i] == '\'' {
						i++
						continue
					}
					break
				}
				i++
			}
			tokens = append(tokens, sqlToken{kind: sqlTokenSingleQuote, text: sql[start:i], end: i})

		case sql[i] == '"':
			start := i
			i++
			for i < n && sql[i] != '"' {
				i++
			}
			if i < n {
				i++
			}
			tokens = append(tokens, sqlToken{kind: sqlTokenDoubleQuote, text: sql[start:i], end: i})

		case sql[i] == '$':
			j := i + 1
			for j < n {
				ch := sql[j]
				if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
					j++
					continue
				}
				break
			}
			if j < n && sql[j] == '$' {
				tag := sql[i : j+1]
				tagLen := len(tag)
				start := i
				i = j + 1
				for i+tagLen <= n {
					if sql[i:i+tagLen] == tag {
						i += tagLen
						break
					}
					i++
				}
				tokens = append(tokens, sqlToken{kind: sqlTokenDollarQuote, text: sql[start:i], end: i})
				continue
			}
			fallthrough

		default:
			start := i
			for i < n {
				c := sql[i]
				if c == '\'' || c == '"' || c == '?' || (c == '-' && i+1 < n && sql[i+1] == '-') || (c == '/' && i+1 < n && sql[i+1] == '*') || c == '$' {
					break
				}
				i++
			}
			if i > start {
				tokens = append(tokens, sqlToken{kind: sqlTokenText, text: sql[start:i], end: i})
			} else if i < n && sql[i] == '?' {
				tokens = append(tokens, sqlToken{kind: sqlTokenPlaceholder, text: "?", end: i + 1})
				i++
			} else if i < n {
				i++
			}
		}
	}
	return tokens
}

func countQuestionPlaceholders(sql string, allowBackslashEscape bool) int {
	if strings.IndexByte(sql, '?') < 0 {
		return 0
	}
	tokens := tokenizeSQL(sql, allowBackslashEscape)
	count := 0
	for _, t := range tokens {
		if t.kind == sqlTokenPlaceholder {
			count++
		}
	}
	return count
}

func rewritePlaceholders(sql string, startIndex int, placeholder func(int) string, isPostgres bool, allowBackslashEscape bool) (string, int, error) {
	if strings.IndexByte(sql, '?') < 0 {
		return sql, startIndex, nil
	}

	tokens := tokenizeSQL(sql, allowBackslashEscape)

	hasPlaceholder := false
	for _, t := range tokens {
		if t.kind == sqlTokenPlaceholder {
			hasPlaceholder = true
			break
		}
	}
	if !hasPlaceholder {
		return sql, startIndex, nil
	}

	estimatedLen := len(sql)
	for _, t := range tokens {
		if t.kind == sqlTokenPlaceholder {
			estimatedLen += 6
		}
	}
	var out strings.Builder
	out.Grow(estimatedLen)

	nextIndex := startIndex
	for _, t := range tokens {
		switch t.kind {
		case sqlTokenPlaceholder:
			if isPostgres {
				afterIdx := t.end
				if afterIdx < len(sql) {
					j := afterIdx
					for j < len(sql) {
						ch := sql[j]
						if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
							j++
							continue
						}
						break
					}
					if j < len(sql) {
						switch sql[j] {
						case '|', '&':
							return "", startIndex, errPGJsonbArrayOp
						case '\'', '"':
							return "", startIndex, errPGJsonbOp
						}
					}
				}
			}
			out.WriteString(placeholder(nextIndex))
			nextIndex++
		default:
			out.WriteString(t.text)
		}
	}

	return out.String(), nextIndex, nil
}

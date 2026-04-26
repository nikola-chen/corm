package builder

import (
	"errors"
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
		return errors.New("corm: missing placeholders for args")
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

var errSQLTooLong = errors.New("corm: SQL statement exceeds maximum length of 1MB")

func (a *argBuilder) checkAndWrite(sql string) error {
	if a.buf.Len()+len(sql) > maxSQLLength {
		return errSQLTooLong
	}
	a.buf.WriteString(sql)
	return nil
}

func countQuestionPlaceholders(sql string, allowBackslashEscape bool) int {
	if strings.IndexByte(sql, '?') < 0 {
		return 0
	}
	inSingleQuote := false
	inDoubleQuote := false
	inLineComment := false
	inBlockComment := false
	dollarTag := ""

	i := 0
	n := len(sql)
	count := 0

	for i < n {
		if inLineComment {
			ch := sql[i]
			i++
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if i+1 < n && sql[i] == '*' && sql[i+1] == '/' {
				i += 2
				inBlockComment = false
				continue
			}
			i++
			continue
		}
		if dollarTag != "" {
			if strings.HasPrefix(sql[i:], dollarTag) {
				i += len(dollarTag)
				dollarTag = ""
				continue
			}
			i++
			continue
		}
		if inSingleQuote {
			ch := sql[i]
			i++
			if allowBackslashEscape && ch == '\\' {
				if i < n {
					i++
				}
				continue
			}
			if ch == '\'' {
				if i < n && sql[i] == '\'' {
					i++
				} else {
					inSingleQuote = false
				}
			}
			continue
		}
		if inDoubleQuote {
			ch := sql[i]
			i++
			if ch == '"' {
				inDoubleQuote = false
			}
			continue
		}

		if i+1 < n && sql[i] == '-' && sql[i+1] == '-' {
			i += 2
			inLineComment = true
			continue
		}
		if i+1 < n && sql[i] == '/' && sql[i+1] == '*' {
			i += 2
			inBlockComment = true
			continue
		}
		if sql[i] == '\'' {
			i++
			inSingleQuote = true
			continue
		}
		if sql[i] == '"' {
			i++
			inDoubleQuote = true
			continue
		}
		if sql[i] == '$' {
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
				i = j + 1
				dollarTag = tag
				continue
			}
		}

		if sql[i] == '?' {
			count++
		}
		i++
	}
	return count
}

func rewritePlaceholders(sql string, startIndex int, placeholder func(int) string, isPostgres bool, allowBackslashEscape bool) (string, int, error) {
	if strings.IndexByte(sql, '?') < 0 {
		return sql, startIndex, nil
	}

	isSimple := true
	for i := 0; i < len(sql); i++ {
		c := sql[i]
		if c == '\'' || c == '"' || c == '-' || c == '/' || c == '$' {
			isSimple = false
			break
		}
	}

	if isSimple {
		count := 0
		for i := 0; i < len(sql); i++ {
			if sql[i] == '?' {
				count++
			}
		}
		if count == 0 {
			return sql, startIndex, nil
		}
		var out strings.Builder
		out.Grow(len(sql) + count*4)
		nextIndex := startIndex
		for i := 0; i < len(sql); i++ {
			if sql[i] == '?' {
				out.WriteString(placeholder(nextIndex))
				nextIndex++
			} else {
				out.WriteByte(sql[i])
			}
		}
		return out.String(), nextIndex, nil
	}

	var out strings.Builder
	out.Grow(len(sql) + 8)

	inSingleQuote := false
	inDoubleQuote := false
	inLineComment := false
	inBlockComment := false
	dollarTag := ""

	i := 0
	n := len(sql)
	nextIndex := startIndex

	for i < n {
		if inLineComment {
			ch := sql[i]
			out.WriteByte(ch)
			i++
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if i+1 < n && sql[i] == '*' && sql[i+1] == '/' {
				out.WriteString("*/")
				i += 2
				inBlockComment = false
				continue
			}
			out.WriteByte(sql[i])
			i++
			continue
		}
		if dollarTag != "" {
			if strings.HasPrefix(sql[i:], dollarTag) {
				out.WriteString(dollarTag)
				i += len(dollarTag)
				dollarTag = ""
				continue
			}
			out.WriteByte(sql[i])
			i++
			continue
		}
		if inSingleQuote {
			ch := sql[i]
			out.WriteByte(ch)
			i++
			if allowBackslashEscape && ch == '\\' {
				if i < n {
					out.WriteByte(sql[i])
					i++
				}
				continue
			}
			if ch == '\'' {
				if i < n && sql[i] == '\'' {
					out.WriteByte(sql[i])
					i++
				} else {
					inSingleQuote = false
				}
			}
			continue
		}
		if inDoubleQuote {
			ch := sql[i]
			out.WriteByte(ch)
			i++
			if ch == '"' {
				inDoubleQuote = false
			}
			continue
		}

		if i+1 < n && sql[i] == '-' && sql[i+1] == '-' {
			out.WriteString("--")
			i += 2
			inLineComment = true
			continue
		}
		if i+1 < n && sql[i] == '/' && sql[i+1] == '*' {
			out.WriteString("/*")
			i += 2
			inBlockComment = true
			continue
		}
		if sql[i] == '\'' {
			out.WriteByte('\'')
			i++
			inSingleQuote = true
			continue
		}
		if sql[i] == '"' {
			out.WriteByte('"')
			i++
			inDoubleQuote = true
			continue
		}
		if sql[i] == '$' {
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
				out.WriteString(tag)
				i = j + 1
				dollarTag = tag
				continue
			}
		}

		if sql[i] != '?' {
			out.WriteByte(sql[i])
			i++
			continue
		}

		if isPostgres {
			j := i + 1
			for j < n {
				ch := sql[j]
				if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
					j++
					continue
				}
				break
			}
			if j < n {
				switch sql[j] {
				case '|', '&':
					return "", startIndex, errors.New("corm: postgres jsonb operator '?|/?&' conflicts with placeholder '?', use jsonb_exists_any/jsonb_exists_all")
				case '\'', '"':
					return "", startIndex, errors.New("corm: postgres jsonb operator '?' conflicts with placeholder '?', use jsonb_exists")
				}
			}
		}

		out.WriteString(placeholder(nextIndex))
		nextIndex++
		i++
	}

	return out.String(), nextIndex, nil
}

package builder

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/dialect"
)

type argBuilder struct {
	d         dialect.Dialect
	idx       int
	args      []any
	usesQmark bool
}

func newArgBuilder(d dialect.Dialect, startIndex int) *argBuilder {
	return &argBuilder{
		d:         d,
		idx:       startIndex,
		args:      make([]any, 0, 16),
		usesQmark: d.Placeholder(1) == "?",
	}
}

func (a *argBuilder) usesQuestionPlaceholders() bool {
	return a.usesQmark
}

func (a *argBuilder) add(v any) string {
	a.args = append(a.args, v)
	p := a.d.Placeholder(a.idx)
	a.idx++
	return p
}

func (a *argBuilder) appendExpr(buf *bytes.Buffer, e clause.Expr) error {
	if strings.TrimSpace(e.SQL) == "" {
		return nil
	}
	if len(e.Args) == 0 {
		buf.WriteString(e.SQL)
		return nil
	}
	if strings.IndexByte(e.SQL, '?') < 0 {
		return errors.New("corm: missing placeholders for args")
	}
	expected := len(e.Args)
	if a.usesQuestionPlaceholders() {
		if count := countQuestionPlaceholders(e.SQL, a.d.Name() == "mysql"); count != expected {
			return fmt.Errorf("corm: placeholder count mismatch: expected %d, got %d", expected, count)
		}
		buf.WriteString(e.SQL)
		a.args = append(a.args, e.Args...)
		a.idx += expected
		return nil
	}
	if a.d.Name() == "postgres" {
		rewritten, next, err := rewritePostgresQuestionPlaceholders(e.SQL, a.idx, false)
		if err != nil {
			return err
		}
		if next-a.idx != expected {
			return fmt.Errorf("corm: placeholder count mismatch: expected %d, got %d", expected, next-a.idx)
		}
		buf.WriteString(rewritten)
		a.args = append(a.args, e.Args...)
		a.idx = next
		return nil
	}
	rewritten, next, err := rewriteQuestionPlaceholders(e.SQL, a.idx, a.d.Placeholder, a.d.Name() == "mysql")
	if err != nil {
		return err
	}
	if next-a.idx != expected {
		return fmt.Errorf("corm: placeholder count mismatch: expected %d, got %d", expected, next-a.idx)
	}
	buf.WriteString(rewritten)
	a.args = append(a.args, e.Args...)
	a.idx = next
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

func rewriteQuestionPlaceholders(sql string, startIndex int, placeholder func(int) string, allowBackslashEscape bool) (string, int, error) {
	if strings.IndexByte(sql, '?') < 0 {
		return sql, startIndex, nil
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

		out.WriteString(placeholder(nextIndex))
		nextIndex++
		i++
	}

	return out.String(), nextIndex, nil
}

func rewritePlaceholdersCommon(sql string, startIndex int, isPostgres bool) (string, int, error) {
	if strings.IndexByte(sql, '?') < 0 && !isPostgres {
		return sql, startIndex, nil
	}
	var out strings.Builder
	out.Grow(len(sql) + 8)

	inSingleQuote := false
	inDoubleQuote := false
	inLineComment := false
	inBlockComment := false
	dollarTag := ""
	isMySQL := !isPostgres

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
			if isMySQL && ch == '\\' && i < n {
				out.WriteByte(sql[i])
				i++
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

		out.WriteByte('$')
		out.WriteString(strconv.Itoa(nextIndex))
		nextIndex++
		i++
	}

	return out.String(), nextIndex, nil
}

func rewritePostgresQuestionPlaceholders(sql string, startIndex int, allowBackslashEscape bool) (string, int, error) {
	return rewritePlaceholdersCommon(sql, startIndex, true)
}

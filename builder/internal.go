package builder

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/nikola-chen/corm/dialect"
)

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

const maxPooledBufferCap = 64 * 1024

func getBuffer() *bytes.Buffer {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func putBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	if buf.Cap() > maxPooledBufferCap {
		return
	}
	bufferPool.Put(buf)
}

func isSimpleIdent(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 0 {
			if c != '_' && !isASCIILetter(c) {
				// First character must be letter or underscore
				if c >= 0x80 {
					// Non-ASCII: use unicode for first char
					r, _ := utf8.DecodeRuneInString(s)
					return r != utf8.RuneError && unicode.IsLetter(r)
				}
				return false
			}
			continue
		}
		if c != '_' && !isASCIILetter(c) && !isASCIIDigit(c) {
			// Non-ASCII in middle: use unicode
			if c >= 0x80 {
				return isSimpleIdentUnicode(s)
			}
			return false
		}
	}
	return true
}

// isASCIILetter reports whether c is an ASCII letter.
func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isASCIIDigit reports whether c is an ASCII digit.
func isASCIIDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// isSimpleIdentUnicode checks if s is a simple identifier using unicode.
// Called as fallback when non-ASCII characters are detected.
func isSimpleIdentUnicode(s string) bool {
	for i, r := range s {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func quoteSelectColumnStrict(d dialect.Dialect, ident string) (string, bool) {
	ident = strings.TrimSpace(ident)
	if ident == "" {
		return "", false
	}
	if ident == "*" {
		return "*", true
	}
	if d == nil {
		return "", false
	}
	// Fast check for invalid chars
	if strings.ContainsAny(ident, " ()+-/*,%<>=!|&^~?:;\"`") {
		return "", false
	}

	// Optimization: avoid strings.Split if no dot present
	if strings.IndexByte(ident, '.') == -1 {
		if !isSimpleIdent(ident) {
			return "", false
		}
		return d.QuoteIdent(ident), true
	}

	// Manual split-and-quote using buffer to avoid slice allocation
	var buf strings.Builder
	buf.Grow(len(ident) + 4) // heuristic: 2 quotes per part

	start := 0
	for i := 0; i <= len(ident); i++ {
		if i == len(ident) || ident[i] == '.' {
			part := ident[start:i]
			// part = strings.TrimSpace(part) // parts inside dot usually don't have spaces if simpleIdent check passes
			if part == "" {
				return "", false
			}

			if buf.Len() > 0 {
				buf.WriteByte('.')
			}

			if part == "*" {
				if i != len(ident) { // * must be last part
					return "", false
				}
				buf.WriteString("*")
			} else {
				if !isSimpleIdent(part) {
					return "", false
				}
				buf.WriteString(d.QuoteIdent(part))
			}
			start = i + 1
		}
	}
	return buf.String(), true
}

func quoteTableStrict(d dialect.Dialect, ident string) (string, bool) {
	return quoteIdentStrict(d, ident)
}

func quoteIdentStrict(d dialect.Dialect, ident string) (string, bool) {
	ident = strings.TrimSpace(ident)
	if ident == "" {
		return "", false
	}
	if d == nil {
		return "", false
	}
	if strings.ContainsAny(ident, " ()+-/*,%<>=!|&^~?:;\"`") {
		return "", false
	}

	// Optimization: avoid strings.Split if no dot present
	if strings.IndexByte(ident, '.') == -1 {
		if !isSimpleIdent(ident) {
			return "", false
		}
		return d.QuoteIdent(ident), true
	}

	// Manual split-and-quote using buffer
	var buf strings.Builder
	buf.Grow(len(ident) + 4)

	start := 0
	for i := 0; i <= len(ident); i++ {
		if i == len(ident) || ident[i] == '.' {
			part := ident[start:i]
			if !isSimpleIdent(part) {
				return "", false
			}
			if buf.Len() > 0 {
				buf.WriteByte('.')
			}
			buf.WriteString(d.QuoteIdent(part))
			start = i + 1
		}
	}
	return buf.String(), true
}

func validateTable(d dialect.Dialect, table string) (string, error) {
	if strings.TrimSpace(table) == "" {
		return "", errors.New("corm: missing table name")
	}
	if d == nil {
		return "", errors.New("corm: nil dialect")
	}
	qTable, ok := quoteTableStrict(d, table)
	if !ok {
		return "", errors.New("corm: invalid table identifier")
	}
	return qTable, nil
}

func quoteColumnStrict(d dialect.Dialect, column string) (string, bool) {
	column = strings.TrimSpace(column)
	if column == "" || column == "*" || strings.Contains(column, ".") {
		return "", false
	}
	if d == nil {
		return "", false
	}
	if strings.ContainsAny(column, " ()+-/*,%<>=!|&^~?:;") {
		return "", false
	}
	if strings.Contains(column, "\"") || strings.Contains(column, "`") {
		return "", false
	}
	if !isSimpleIdent(column) {
		return "", false
	}
	return d.QuoteIdent(column), true
}

func normalizeSubqueryOp(op string) (string, bool) {
	op = strings.TrimSpace(strings.ToUpper(op))
	switch op {
	case "=", "!=", "<>", ">", "<", ">=", "<=", "IN", "NOT IN", "LIKE", "NOT LIKE":
		return op, true
	default:
		return "", false
	}
}

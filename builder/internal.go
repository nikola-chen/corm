package builder

import (
	"errors"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/nikola-chen/corm/dialect"
)

var stringBuilderPool = sync.Pool{
	New: func() any {
		b := new(strings.Builder)
		b.Grow(512)
		return b
	},
}

const maxPooledStringBuilderCap = 64 * 1024

func getBuffer() *strings.Builder {
	buf := stringBuilderPool.Get().(*strings.Builder)
	buf.Reset()
	// Only grow if capacity is too small
	if buf.Cap() < 512 {
		buf.Grow(512)
	}
	return buf
}

func putBuffer(buf *strings.Builder) {
	if buf == nil {
		return
	}
	if buf.Cap() > maxPooledStringBuilderCap {
		return
	}
	stringBuilderPool.Put(buf)
}

func isSimpleIdent(s string) bool {
	if len(s) == 0 {
		return false
	}

	// Check first character
	c := s[0]
	if c != '_' && !isASCIILetter(c) {
		if c >= 0x80 {
			r, _ := utf8.DecodeRuneInString(s)
			return r != utf8.RuneError && unicode.IsLetter(r)
		}
		return false
	}

	// Check remaining characters
	for i := 1; i < len(s); i++ {
		c := s[i]
		if c != '_' && !isASCIILetter(c) && !isASCIIDigit(c) {
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
	return quoteIdentWithStar(d, ident, true)
}

func quoteTableStrict(d dialect.Dialect, ident string) (string, bool) {
	return quoteIdentStrict(d, ident)
}

func quoteIdentStrict(d dialect.Dialect, ident string) (string, bool) {
	return quoteIdentWithStar(d, ident, false)
}

func quoteIdentWithStar(d dialect.Dialect, ident string, allowStar bool) (string, bool) {
	// Inline TrimSpace to avoid function call overhead
	start := 0
	for start < len(ident) && (ident[start] == ' ' || ident[start] == '\t' || ident[start] == '\n' || ident[start] == '\r') {
		start++
	}
	end := len(ident)
	for end > start && (ident[end-1] == ' ' || ident[end-1] == '\t' || ident[end-1] == '\n' || ident[end-1] == '\r') {
		end--
	}
	ident = ident[start:end]

	if ident == "" {
		return "", false
	}
	if d == nil {
		return "", false
	}

	if allowStar && ident == "*" {
		return "*", true
	}

	// Inline ContainsAny check for better performance
	for i := 0; i < len(ident); i++ {
		c := ident[i]
		if c == ' ' || c == '(' || c == ')' || c == '+' || c == '-' || c == '/' || c == '*' ||
			c == ',' || c == '%' || c == '<' || c == '>' || c == '=' || c == '!' || c == '|' ||
			c == '&' || c == '^' || c == '~' || c == '?' || c == ':' || c == ';' || c == '"' || c == '`' {
			return "", false
		}
	}

	dotIdx := strings.IndexByte(ident, '.')
	if dotIdx == -1 {
		if !isSimpleIdent(ident) {
			return "", false
		}
		return d.QuoteIdent(ident), true
	}

	// Handle table.column format without Split
	part1 := ident[:dotIdx]
	part2 := ident[dotIdx+1:]

	if part1 == "" || part2 == "" {
		return "", false
	}

	if !isSimpleIdent(part1) {
		return "", false
	}

	var result strings.Builder
	result.Grow(len(ident) + 4)
	result.WriteString(d.QuoteIdent(part1))
	result.WriteByte('.')

	if allowStar && part2 == "*" {
		result.WriteByte('*')
	} else {
		if !isSimpleIdent(part2) {
			return "", false
		}
		result.WriteString(d.QuoteIdent(part2))
	}
	return result.String(), true
}

func validateTable(d dialect.Dialect, table string) (string, error) {
	if strings.TrimSpace(table) == "" {
		return "", errors.New("corm: missing table name")
	}
	if d == nil {
		return "", errors.New("corm: nil dialect")
	}
	// Apply same length limit as SAVEPOINT (128 chars) for consistency
	if len(table) > 128 {
		return "", errors.New("corm: table name exceeds maximum length of 128 characters")
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

package builder

import (
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/nikola-chen/corm/dialect"
)

var bufPool = sync.Pool{
	New: func() any {
		b := new(strings.Builder)
		b.Grow(512)
		return b
	},
}

func getBuffer() *strings.Builder {
	buf := bufPool.Get().(*strings.Builder)
	return buf
}

func putBuffer(buf *strings.Builder) {
	buf.Reset()
	bufPool.Put(buf)
}

// trimSpaceASCII trims ASCII whitespace from both ends of s.
// It is faster than strings.TrimSpace for ASCII-only strings.
func trimSpaceASCII(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
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

var specialChars [256]bool

func init() {
	for _, c := range " ()+-/*,%<>=!|&^~?:;\"`" {
		specialChars[c] = true
	}
}

func quoteIdentWithStar(d dialect.Dialect, ident string, allowStar bool) (string, bool) {
	ident = trimSpaceASCII(ident)

	if ident == "" {
		return "", false
	}
	if d == nil {
		return "", false
	}

	if allowStar && ident == "*" {
		return "*", true
	}

	for i := range ident {
		if specialChars[ident[i]] {
			return "", false
		}
	}

	before, after, ok := strings.Cut(ident, ".")
	if !ok {
		if !isSimpleIdent(ident) {
			return "", false
		}
		return d.QuoteIdent(ident), true
	}

	// Handle table.column format without Split
	part1 := before
	part2 := after

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
		return "", errMissingTable
	}
	if d == nil {
		return "", errNilDialect
	}
	if len(table) > 128 {
		return "", errTableNameTooLong
	}
	qTable, ok := quoteTableStrict(d, table)
	if !ok {
		return "", errInvalidTable
	}
	return qTable, nil
}

func quoteColumnStrict(d dialect.Dialect, column string) (string, bool) {
	column = strings.TrimSpace(column)
	if column == "" || column == "*" || strings.ContainsRune(column, '.') {
		return "", false
	}
	if d == nil {
		return "", false
	}
	if !isSimpleIdent(column) {
		return "", false
	}
	return d.QuoteIdent(column), true
}

func normalizeSubqueryOp(op string) (string, bool) {
	op = strings.ToUpper(strings.TrimSpace(op))
	switch op {
	case "=", "!=", "<>", ">", "<", ">=", "<=", "IN", "NOT IN", "LIKE", "NOT LIKE":
		return op, true
	default:
		return "", false
	}
}

package builder

import (
	"strings"
	"unicode"

	"github.com/nikola-chen/corm/dialect"
)

func quoteMaybe(d dialect.Dialect, ident string) string {
	ident = strings.TrimSpace(ident)
	if ident == "" {
		return ident
	}
	if ident == "*" {
		return "*"
	}
	if strings.ContainsAny(ident, " ()+-/*,%<>=!|&^~?:;") {
		return ident
	}
	if strings.Contains(ident, "\"") || strings.Contains(ident, "`") {
		return ident
	}

	parts := strings.Split(ident, ".")
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "*" {
			quoted = append(quoted, "*")
			continue
		}
		if !isSimpleIdent(p) {
			return ident
		}
		quoted = append(quoted, d.QuoteIdent(p))
	}
	return strings.Join(quoted, ".")
}

func isSimpleIdent(s string) bool {
	if s == "" {
		return false
	}
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

func quoteIdentStrict(d dialect.Dialect, ident string) (string, bool) {
	ident = strings.TrimSpace(ident)
	if ident == "" {
		return "", false
	}
	if ident == "*" {
		return "*", true
	}
	if strings.ContainsAny(ident, " ()+-/*,%<>=!|&^~?:;") {
		return "", false
	}
	if strings.Contains(ident, "\"") || strings.Contains(ident, "`") {
		return "", false
	}

	parts := strings.Split(ident, ".")
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if !isSimpleIdent(p) {
			return "", false
		}
		quoted = append(quoted, d.QuoteIdent(p))
	}
	return strings.Join(quoted, "."), true
}

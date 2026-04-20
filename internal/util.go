package internal

import "strings"

func NormalizeColumn(c string) string {
	start := 0
	end := len(c)
	for start < end && (c[start] == ' ' || c[start] == '\t' || c[start] == '\n' || c[start] == '\r') {
		start++
	}
	for end > start && (c[end-1] == ' ' || c[end-1] == '\t' || c[end-1] == '\n' || c[end-1] == '\r') {
		end--
	}
	c = c[start:end]

	if c == "" {
		return ""
	}

	lastDot := -1
	needsLower := false
	needsStrip := false
	hasNonASCII := false
	for i := 0; i < len(c); i++ {
		ch := c[i]
		if ch == '.' {
			lastDot = i
		}
		if ch == '`' || ch == '"' {
			needsStrip = true
		}
		if ch >= 'A' && ch <= 'Z' {
			needsLower = true
		}
		if ch >= 0x80 {
			hasNonASCII = true
		}
	}

	if lastDot >= 0 {
		c = c[lastDot+1:]
	}

	if c == "" {
		return ""
	}

	if !needsStrip && !needsLower && !hasNonASCII {
		return c
	}

	if hasNonASCII {
		return normalizeColumnUnicode(c, needsStrip)
	}

	if !needsStrip {
		return toLowerASCII(c)
	}

	var b strings.Builder
	b.Grow(len(c))
	for i := 0; i < len(c); i++ {
		ch := c[i]
		if ch == '`' || ch == '"' {
			continue
		}
		if ch >= 'A' && ch <= 'Z' {
			ch += 'a' - 'A'
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func normalizeColumnUnicode(c string, needsStrip bool) string {
	if needsStrip {
		var b strings.Builder
		b.Grow(len(c))
		for _, r := range c {
			if r == '`' || r == '"' {
				continue
			}
			b.WriteRune(r)
		}
		c = b.String()
	}
	return strings.ToLower(c)
}

func toLowerASCII(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch >= 'A' && ch <= 'Z' {
			ch += 'a' - 'A'
		}
		b.WriteByte(ch)
	}
	return b.String()
}

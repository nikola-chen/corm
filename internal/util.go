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
	for i := range c {
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

	buf := make([]byte, 0, len(c))
	for i := range c {
		ch := c[i]
		if ch == '`' || ch == '"' {
			continue
		}
		if ch >= 'A' && ch <= 'Z' {
			ch += 'a' - 'A'
		}
		buf = append(buf, ch)
	}
	return string(buf)
}

func normalizeColumnUnicode(c string, needsStrip bool) string {
	if needsStrip {
		buf := make([]rune, 0, len(c))
		for _, r := range c {
			if r == '`' || r == '"' {
				continue
			}
			buf = append(buf, r)
		}
		c = string(buf)
	}
	return strings.ToLower(c)
}

func toLowerASCII(s string) string {
	buf := make([]byte, len(s))
	for i := range s {
		ch := s[i]
		if ch >= 'A' && ch <= 'Z' {
			ch += 'a' - 'A'
		}
		buf[i] = ch
	}
	return string(buf)
}

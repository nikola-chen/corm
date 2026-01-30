// Package internal provides internal utilities for corm.
// This package is not part of the public API and may change without notice.
package internal

import "strings"

// NormalizeColumn normalizes a column name by:
// 1. Trimming whitespace
// 2. Removing all quote characters (` and ")
// 3. Extracting the column name after the last dot (table.column -> column)
// 4. Converting to lowercase for case-insensitive comparison
func NormalizeColumn(c string) string {
	c = strings.TrimSpace(c)
	// Remove all backticks and double quotes
	c = strings.ReplaceAll(c, "`", "")
	c = strings.ReplaceAll(c, "\"", "")
	if i := strings.LastIndexByte(c, '.'); i >= 0 {
		c = c[i+1:]
	}
	return strings.ToLower(c)
}

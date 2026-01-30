package internal

import (
	"testing"
)

func TestNormalizeColumn(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Basic cases
		{"id", "id"},
		{"name", "name"},
		{"user_id", "user_id"},

		// With whitespace
		{"  id  ", "id"},
		{"\tname\n", "name"},

		// With quotes
		{"`id`", "id"},
		{"\"name\"", "name"},
		{"`user_id`", "user_id"},

		// With table prefix
		{"users.id", "id"},
		{"users.name", "name"},
		{"db.users.id", "id"},

		// With quotes and prefix
		{"`users`.`id`", "id"},
		{"\"users\".\"name\"", "name"},

		// Case conversion
		{"ID", "id"},
		{"Name", "name"},
		{"USER_ID", "user_id"},
		{"Users.ID", "id"},

		// Mixed cases
		{"  `Users`.`ID`  ", "id"},
		{"\"DB\".\"Users\".\"Name\"", "name"},

		// Empty and edge cases
		{"", ""},
		{"`", ""},
		{"\"", ""},
		{".", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeColumn(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeColumn(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeColumnUnicode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Unicode column names (should work correctly)
		{"用户名", "用户名"},
		{"用户.用户名", "用户名"},
		{"`用户名`", "用户名"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeColumn(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeColumn(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func BenchmarkNormalizeColumn(b *testing.B) {
	columns := []string{
		"id",
		"user_id",
		"  name  ",
		"`email`",
		"users.created_at",
		"`db`.`table`.`column`",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, col := range columns {
			_ = NormalizeColumn(col)
		}
	}
}

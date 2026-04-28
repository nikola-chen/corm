package engine

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestFormatArgsDefaultRedaction(t *testing.T) {
	s := strings.Repeat("a", 33)
	out := formatArgs([]any{s, []byte{1, 2, 3}}, nil, 0, 0)
	if !strings.Contains(out, "redacted(len=33)") {
		t.Fatalf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "bytes(len=3)") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFormatArgsCustomFormatter(t *testing.T) {
	out := formatArgs([]any{1, "x"}, func(any) string { return "X" }, 0, 0)
	if out != "[X, X]" {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFormatArgsMaxItems(t *testing.T) {
	args := make([]any, 25)
	for i := range args {
		args[i] = i
	}
	out := formatArgs(args, nil, 3, 0)
	if !strings.Contains(out, "…") {
		t.Fatalf("expected truncation indicator, got: %s", out)
	}
}

func TestFormatArgsMaxLen(t *testing.T) {
	args := make([]any, 10)
	for i := range args {
		args[i] = strings.Repeat("a", 50)
	}
	out := formatArgs(args, nil, 0, 100)
	if len(out) > 150 {
		t.Fatalf("expected truncated output, got len=%d: %s", len(out), out)
	}
}

func TestFormatArgsEmpty(t *testing.T) {
	out := formatArgs(nil, nil, 0, 0)
	if out != "[]" {
		t.Fatalf("expected empty array, got: %s", out)
	}
}

func TestTruncateSQL(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		maxLen   int
		expected string
	}{
		{"short", "SELECT * FROM users", 100, "SELECT * FROM users"},
		{"exact", "SELECT * FROM users", 19, "SELECT * FROM users"},
		{"long", "SELECT * FROM users WHERE name = 'test'", 20, "SELECT * FROM users …"},
		{"default", strings.Repeat("a", 3000), 0, strings.Repeat("a", 2048) + "…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateSQL(tt.sql, tt.maxLen)
			if got != tt.expected {
				t.Errorf("truncateSQL() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDefaultArgFormatter(t *testing.T) {
	tests := []struct {
		name     string
		arg      any
		expected string
	}{
		{"nil", nil, "null"},
		{"string", "hello", "redacted(len=5)"},
		{"bytes", []byte{1, 2, 3}, "bytes(len=3)"},
		{"int", 42, "42"},
		{"float", 3.14, "3.14"},
		{"bool", true, "true"},
		{"error", context.Canceled, "redacted)"},
		{"stringer", time.Second, "Duration(redacted)"},
		{"struct", struct{ Name string }{"test"}, "{test}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultArgFormatter(tt.arg)
			if !strings.Contains(got, tt.expected) && got != tt.expected {
				t.Errorf("defaultArgFormatter() = %q, want containing %q", got, tt.expected)
			}
		})
	}
}

func TestDefaultArgFormatterLongValue(t *testing.T) {
	longStr := struct{ S string }{S: strings.Repeat("x", 100)}
	got := defaultArgFormatter(longStr)
	if !strings.Contains(got, "…") {
		t.Errorf("long value should be truncated, got: %s", got)
	}
}

type mockExecutor struct {
	execCalled  bool
	queryCalled bool
}

func (m *mockExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	m.execCalled = true
	return nil, nil
}

func (m *mockExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	m.queryCalled = true
	return nil, nil
}

func TestLoggingExecutorExecContext(t *testing.T) {
	mock := &mockExecutor{}
	le := &loggingExecutor{
		inner:  mock,
		logger: NopLogger{},
		cfg: Config{
			LogSQL:  true,
			LogArgs: true,
		},
	}

	_, _ = le.ExecContext(context.Background(), "INSERT INTO users (name) VALUES (?)", "test")
	if !mock.execCalled {
		t.Error("expected inner ExecContext to be called")
	}
}

func TestLoggingExecutorQueryContext(t *testing.T) {
	mock := &mockExecutor{}
	le := &loggingExecutor{
		inner:  mock,
		logger: NopLogger{},
		cfg: Config{
			LogSQL:  true,
			LogArgs: true,
		},
	}

	_, _ = le.QueryContext(context.Background(), "SELECT * FROM users WHERE id = ?", 1)
	if !mock.queryCalled {
		t.Error("expected inner QueryContext to be called")
	}
}

func TestLoggingExecutorSlowQuery(t *testing.T) {
	mock := &mockExecutor{}
	le := &loggingExecutor{
		inner:  mock,
		logger: NopLogger{},
		cfg: Config{
			SlowQuery: time.Microsecond,
		},
	}

	_, _ = le.ExecContext(context.Background(), "SELECT 1")
	if !mock.execCalled {
		t.Error("expected inner ExecContext to be called")
	}
}

func TestLoggingExecutorNoLog(t *testing.T) {
	mock := &mockExecutor{}
	le := &loggingExecutor{
		inner:  mock,
		logger: nil,
		cfg: Config{
			LogSQL: false,
		},
	}

	_, _ = le.ExecContext(context.Background(), "SELECT 1")
	if !mock.execCalled {
		t.Error("expected inner ExecContext to be called even without logger")
	}
}

package engine

import (
	"strings"
	"testing"
)

func TestFormatArgsDefaultRedaction(t *testing.T) {
	s := strings.Repeat("a", 33)
	out := formatArgs([]any{s, []byte{1, 2, 3}}, nil)
	if !strings.Contains(out, "redacted(len=33)") {
		t.Fatalf("unexpected output: %s", out)
	}
	if !strings.Contains(out, "bytes(len=3)") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFormatArgsCustomFormatter(t *testing.T) {
	out := formatArgs([]any{1, "x"}, func(any) string { return "X" })
	if out != "[X, X]" {
		t.Fatalf("unexpected output: %s", out)
	}
}


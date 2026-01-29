package corm

import "testing"

func TestWithDBUnsupportedDialect(t *testing.T) {
	if _, err := WithDB(nil, "unsupported-dialect"); err == nil {
		t.Fatalf("expected error")
	}
}

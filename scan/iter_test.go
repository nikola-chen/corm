package scan_test

import (
	"context"
	"testing"

	"github.com/nikola-chen/corm/scan"
)

func TestIterStruct(t *testing.T) {
	db := openTestDB(t)
	rows, err := db.QueryContext(context.Background(), "struct_two_rows")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}

	count := 0
	for u, err := range scan.Iter[User](rows) {
		if err != nil {
			t.Fatalf("Iter error: %v", err)
		}
		count++
		if count == 1 && u.Name != "alice" {
			t.Errorf("expected alice, got %s", u.Name)
		}
		if count == 2 && u.Name != "bob" {
			t.Errorf("expected bob, got %s", u.Name)
		}
	}

	if count != 2 {
		t.Errorf("expected 2 users, got %d", count)
	}
}

func TestIterMap(t *testing.T) {
	db := openTestDB(t)
	rows, err := db.QueryContext(context.Background(), "map_two_rows")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}

	count := 0
	for m, err := range scan.Iter[map[string]any](rows) {
		if err != nil {
			t.Fatalf("Iter error: %v", err)
		}
		count++
		if m["id"] == nil {
			t.Errorf("missing id in map")
		}
	}

	if count != 2 {
		t.Errorf("expected 2 rows, got %d", count)
	}
}

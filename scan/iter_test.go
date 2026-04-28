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

func TestIterEmpty(t *testing.T) {
	db := openTestDB(t)
	rows, err := db.QueryContext(context.Background(), "empty")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}

	count := 0
	for _, err := range scan.Iter[User](rows) {
		if err != nil {
			t.Fatalf("Iter error: %v", err)
		}
		count++
	}

	if count != 0 {
		t.Errorf("expected 0 rows, got %d", count)
	}
}

func TestIterEarlyExit(t *testing.T) {
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
		if u.ID == 1 {
			break
		}
	}

	if count != 1 {
		t.Errorf("expected 1 user after early exit, got %d", count)
	}
}

func TestIterWrongMapKeyType(t *testing.T) {
	db := openTestDB(t)
	rows, err := db.QueryContext(context.Background(), "map_two_rows")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}

	for _, err := range scan.Iter[map[int]any](rows) {
		if err == nil {
			t.Fatal("expected error for non-string map key")
		}
		if err.Error() != "corm: map element must have string keys" {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestIterNonStructNonMap(t *testing.T) {
	db := openTestDB(t)
	rows, err := db.QueryContext(context.Background(), "one_row")
	if err != nil {
		t.Fatalf("QueryContext: %v", err)
	}

	for _, err := range scan.Iter[int](rows) {
		if err == nil {
			t.Fatal("expected error for non-struct/non-map dest")
		}
		if err.Error() != "corm: dest must be struct, *struct, or map" {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

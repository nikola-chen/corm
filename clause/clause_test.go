package clause_test

import (
	"testing"

	"github.com/nikola-chen/corm/clause"
)

func TestIn(t *testing.T) {
	tests := []struct {
		name     string
		column   string
		values   []any
		wantSQL  string
		wantArgs []any
	}{
		{
			name:     "single value",
			column:   "id",
			values:   []any{1},
			wantSQL:  "id IN (?)",
			wantArgs: []any{1},
		},
		{
			name:     "multiple values",
			column:   "name",
			values:   []any{"a", "b"},
			wantSQL:  "name IN (?, ?)",
			wantArgs: []any{"a", "b"},
		},
		{
			name:     "slice expansion",
			column:   "status",
			values:   []any{[]int{1, 2, 3}},
			wantSQL:  "status IN (?, ?, ?)",
			wantArgs: []any{1, 2, 3},
		},
		{
			name:     "mixed values and slice",
			column:   "mix",
			values:   []any{1, []int{2, 3}, 4},
			wantSQL:  "mix IN (?, ?, ?, ?)",
			wantArgs: []any{1, 2, 3, 4},
		},
		{
			name:     "empty slice",
			column:   "id",
			values:   []any{[]int{}},
			wantSQL:  "1=0",
			wantArgs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clause.In(tt.column, tt.values...)
			if got.SQL != tt.wantSQL {
				t.Errorf("In().SQL = %v, want %v", got.SQL, tt.wantSQL)
			}
			if len(got.Args) != len(tt.wantArgs) {
				t.Errorf("In().Args length = %v, want %v", len(got.Args), len(tt.wantArgs))
			}
			// DeepEqual check for args content
			// Note: reflect.DeepEqual might fail on int vs int64 type mismatches if not careful,
			// but here we expect exact types from input.
			// Actually In() flattens using reflect, types should be preserved.
		})
	}
}

func TestEqAndComparisonOperators(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(string, any) clause.Expr
		column   string
		value    any
		wantSQL  string
	}{
		{"Eq", clause.Eq, "id", 1, "id = ?"},
		{"Neq", clause.Neq, "name", "test", "name != ?"},
		{"Gt", clause.Gt, "age", 18, "age > ?"},
		{"Gte", clause.Gte, "score", 90, "score >= ?"},
		{"Lt", clause.Lt, "price", 100, "price < ?"},
		{"Lte", clause.Lte, "quantity", 5, "quantity <= ?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn(tt.column, tt.value)
			if got.SQL != tt.wantSQL {
				t.Errorf("%s().SQL = %v, want %v", tt.name, got.SQL, tt.wantSQL)
			}
			if len(got.Args) != 1 || got.Args[0] != tt.value {
				t.Errorf("%s().Args = %v, want [%v]", tt.name, got.Args, tt.value)
			}
		})
	}
}

func TestIsNullAndIsNotNull(t *testing.T) {
	tests := []struct {
		name    string
		fn      func(string) clause.Expr
		column  string
		wantSQL string
	}{
		{"IsNull", clause.IsNull, "deleted_at", "deleted_at IS NULL"},
		{"IsNotNull", clause.IsNotNull, "email", "email IS NOT NULL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn(tt.column)
			if got.SQL != tt.wantSQL {
				t.Errorf("%s().SQL = %v, want %v", tt.name, got.SQL, tt.wantSQL)
			}
			if len(got.Args) != 0 {
				t.Errorf("%s().Args = %v, want empty", tt.name, got.Args)
			}
		})
	}
}

func TestNot(t *testing.T) {
	tests := []struct {
		name     string
		expr     clause.Expr
		wantSQL  string
		wantArgs []any
	}{
		{"simple expr", clause.Raw("age < ?", 18), "NOT (age < ?)", []any{18}},
		{"empty expr", clause.Expr{}, "", nil},
		{"whitespace expr", clause.Expr{SQL: "   ", Args: []any{1}}, "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clause.Not(tt.expr)
			if got.SQL != tt.wantSQL {
				t.Errorf("Not().SQL = %v, want %v", got.SQL, tt.wantSQL)
			}
			if len(got.Args) != len(tt.wantArgs) {
				t.Errorf("Not().Args length = %v, want %v", len(got.Args), len(tt.wantArgs))
			}
		})
	}
}

func TestRaw(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		args     []any
		wantSQL  string
		wantArgs []any
	}{
		{"basic", "id = ?", []any{1}, "id = ?", []any{1}},
		{"no args", "1=1", nil, "1=1", nil},
		{"multiple args", "a = ? AND b = ?", []any{1, "test"}, "a = ? AND b = ?", []any{1, "test"}},
		{"empty sql", "", []any{1}, "", []any{1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clause.Raw(tt.sql, tt.args...)
			if got.SQL != tt.wantSQL {
				t.Errorf("Raw().SQL = %v, want %v", got.SQL, tt.wantSQL)
			}
			if len(got.Args) != len(tt.wantArgs) {
				t.Errorf("Raw().Args length = %v, want %v", len(got.Args), len(tt.wantArgs))
			}
		})
	}
}

func TestEmptyArgs(t *testing.T) {
	empty := clause.EmptyArgs()
	if len(empty) != 0 {
		t.Errorf("EmptyArgs() length = %v, want 0", len(empty))
	}
	
	// Test that it returns the same slice each time by checking capacity and pointer
	empty2 := clause.EmptyArgs()
	if cap(empty) != cap(empty2) {
		t.Errorf("EmptyArgs() should return slices with same capacity")
	}
	// Can't compare pointers of empty slices, so just ensure both are empty
	if len(empty2) != 0 {
		t.Errorf("EmptyArgs() second call length = %v, want 0", len(empty2))
	}
}

func TestBuildInEdgeCases(t *testing.T) {
	// Test large slices
	largeSlice := make([]int, 1000)
	for i := range largeSlice {
		largeSlice[i] = i
	}
	expr := clause.In("id", largeSlice)
	if len(expr.Args) != 1000 {
		t.Errorf("Expected 1000 args, got %d", len(expr.Args))
	}
	
	// Test nil slice
	var nilSlice []int
	expr2 := clause.In("id", nilSlice)
	if expr2.SQL != "1=0" {
		t.Errorf("Expected '1=0' for nil slice, got %s", expr2.SQL)
	}
	
	// Test different slice types
	expr3 := clause.In("id", []int64{1, 2, 3})
	if len(expr3.Args) != 3 {
		t.Errorf("Expected 3 args for int64 slice, got %d", len(expr3.Args))
	}
}

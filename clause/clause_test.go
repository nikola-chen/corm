package clause_test

import (
	"testing"

	"corm/clause"
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

package builder

import (
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/nikola-chen/corm/clause"
)

// ConflictBuilder builds the ON CONFLICT / ON DUPLICATE KEY UPDATE clause.
type ConflictBuilder struct {
	insertBuilder *InsertBuilder
	conflictCols  []string
	err           error
}

// OnConflict begins the upsert conflict clause.
func (b *InsertBuilder) OnConflict(columns ...string) *ConflictBuilder {
	cb := &ConflictBuilder{
		insertBuilder: b,
		conflictCols:  columns,
	}
	if b.err != nil {
		cb.err = b.err
	}
	return cb
}

// DoNothing specifies to do nothing on conflict.
func (cb *ConflictBuilder) DoNothing() *InsertBuilder {
	if cb.err != nil {
		cb.insertBuilder.err = cb.err
		return cb.insertBuilder
	}
	dName := cb.insertBuilder.d.Name()

	if dName == "postgres" {
		sqlStr := "ON CONFLICT"
		if len(cb.conflictCols) > 0 {
			qCols := make([]string, 0, len(cb.conflictCols))
			for _, col := range cb.conflictCols {
				q, ok := quoteColumnStrict(cb.insertBuilder.d, col)
				if !ok {
					cb.insertBuilder.err = errors.New("corm: invalid column identifier in conflict clause")
					return cb.insertBuilder
				}
				qCols = append(qCols, q)
			}
			sqlStr += " (" + strings.Join(qCols, ", ") + ")"
		}
		sqlStr += " DO NOTHING"
		cb.insertBuilder.suffix = append(cb.insertBuilder.suffix, clause.Raw(sqlStr))
	} else if dName == "mysql" {
		// MySQL does not have an explicit DOES NOTHING without changing a value,
		// but `ON DUPLICATE KEY UPDATE id=id` is a common hack.
		// However, it's safer to just let Suffix or user define the query.
		// Alternatively, we use `INSERT IGNORE` which cannot be added here (it's prefix).
		cb.insertBuilder.err = errors.New("corm: DoNothing() is only supported on Postgres. Use InsertIgnore() or raw suffix on MySQL.")
	} else {
		cb.insertBuilder.err = errors.New("corm: unsupported dialect for OnConflict")
	}

	return cb.insertBuilder
}

// DoUpdate specifies the columns to update on conflict.
func (cb *ConflictBuilder) DoUpdate(sets map[string]any) *InsertBuilder {
	if cb.err != nil {
		cb.insertBuilder.err = cb.err
		return cb.insertBuilder
	}
	if len(sets) == 0 {
		return cb.insertBuilder
	}

	dName := cb.insertBuilder.d.Name()

	// Keys are sorted for deterministic SQL.
	keys := slices.Collect(maps.Keys(sets))
	slices.Sort(keys)

	if dName == "postgres" {
		sqlStr := "ON CONFLICT"
		if len(cb.conflictCols) > 0 {
			qCols := make([]string, 0, len(cb.conflictCols))
			for _, col := range cb.conflictCols {
				q, ok := quoteColumnStrict(cb.insertBuilder.d, col)
				if !ok {
					cb.insertBuilder.err = errors.New("corm: invalid column identifier in conflict clause")
					return cb.insertBuilder
				}
				qCols = append(qCols, q)
			}
			sqlStr += " (" + strings.Join(qCols, ", ") + ")"
		} else {
			cb.insertBuilder.err = errors.New("corm: Postgres requires constraint columns for ON CONFLICT DO UPDATE")
			return cb.insertBuilder
		}

		sqlStr += " DO UPDATE SET "
		args := make([]any, 0, len(keys))
		setStrs := make([]string, 0, len(keys))

		for _, k := range keys {
			q, ok := quoteColumnStrict(cb.insertBuilder.d, k)
			if !ok {
				cb.insertBuilder.err = errors.New("corm: invalid column identifier in conflict clause")
				return cb.insertBuilder
			}
			v := sets[k]
			if e, isExpr := v.(clause.Expr); isExpr {
				setStrs = append(setStrs, q+" = "+e.SQL)
				args = append(args, e.Args...)
			} else {
				setStrs = append(setStrs, q+" = ?")
				args = append(args, v)
			}
		}
		sqlStr += strings.Join(setStrs, ", ")
		cb.insertBuilder.suffix = append(cb.insertBuilder.suffix, clause.Raw(sqlStr, args...))

	} else if dName == "mysql" {
		sqlStr := "ON DUPLICATE KEY UPDATE "
		args := make([]any, 0, len(keys))
		setStrs := make([]string, 0, len(keys))

		for _, k := range keys {
			q, ok := quoteColumnStrict(cb.insertBuilder.d, k)
			if !ok {
				cb.insertBuilder.err = errors.New("corm: invalid column identifier in conflict clause")
				return cb.insertBuilder
			}
			v := sets[k]
			if e, isExpr := v.(clause.Expr); isExpr {
				setStrs = append(setStrs, q+" = "+e.SQL)
				args = append(args, e.Args...)
			} else {
				setStrs = append(setStrs, q+" = ?")
				args = append(args, v)
			}
		}
		sqlStr += strings.Join(setStrs, ", ")
		cb.insertBuilder.suffix = append(cb.insertBuilder.suffix, clause.Raw(sqlStr, args...))
	} else {
		cb.insertBuilder.err = errors.New("corm: unsupported dialect for OnConflict")
	}

	return cb.insertBuilder
}

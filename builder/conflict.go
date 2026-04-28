package builder

import (
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
					cb.insertBuilder.err = errConflictColumn
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
		cb.insertBuilder.err = errConflictDoNothing
	} else {
		cb.insertBuilder.err = errConflictDialect
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
	keys := slices.Collect(maps.Keys(sets))
	slices.Sort(keys)

	if dName == "postgres" {
		if len(cb.conflictCols) == 0 {
			cb.insertBuilder.err = errConflictNoCols
			return cb.insertBuilder
		}
		sqlStr, args, err := cb.buildConflictPrefix("ON CONFLICT", "DO UPDATE SET", keys, sets)
		if err != nil {
			cb.insertBuilder.err = err
			return cb.insertBuilder
		}
		cb.insertBuilder.suffix = append(cb.insertBuilder.suffix, clause.Raw(sqlStr, args...))
	} else if dName == "mysql" {
		sqlStr, args, err := cb.buildSetClause(keys, sets)
		if err != nil {
			cb.insertBuilder.err = err
			return cb.insertBuilder
		}
		sqlStr = "ON DUPLICATE KEY UPDATE " + sqlStr
		cb.insertBuilder.suffix = append(cb.insertBuilder.suffix, clause.Raw(sqlStr, args...))
	} else {
		cb.insertBuilder.err = errConflictDialect
	}

	return cb.insertBuilder
}

func (cb *ConflictBuilder) buildConflictPrefix(prefix, setPrefix string, keys []string, sets map[string]any) (string, []any, error) {
	sqlStr := prefix
	qCols := make([]string, 0, len(cb.conflictCols))
	for _, col := range cb.conflictCols {
		q, ok := quoteColumnStrict(cb.insertBuilder.d, col)
		if !ok {
			return "", nil, errConflictColumn
		}
		qCols = append(qCols, q)
	}
	sqlStr += " (" + strings.Join(qCols, ", ") + ") " + setPrefix + " "

	setSQL, setArgs, err := cb.buildSetClause(keys, sets)
	if err != nil {
		return "", nil, err
	}
	return sqlStr + setSQL, setArgs, nil
}

func (cb *ConflictBuilder) buildSetClause(keys []string, sets map[string]any) (string, []any, error) {
	args := make([]any, 0, len(keys))
	setStrs := make([]string, 0, len(keys))
	for _, k := range keys {
		q, ok := quoteColumnStrict(cb.insertBuilder.d, k)
		if !ok {
			return "", nil, errConflictColumn
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
	return strings.Join(setStrs, ", "), args, nil
}

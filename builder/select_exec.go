package builder

import (
	"context"
	"database/sql"
	"errors"
	"iter"

	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/scan"
)

func (b *SelectBuilder) All(ctx context.Context, dest any) error {
	if b.exec == nil {
		return errors.New("corm: missing Executor for select")
	}
	rows, err := b.Query(ctx)
	if err != nil {
		return err
	}
	hint := 0
	if b.limit != nil && *b.limit > 0 {
		hint = *b.limit
	}
	return scan.ScanAllCap(rows, dest, hint)
}

// Iter returns an iterator that yields elements of type T from the query result.
// T can be a struct, *struct, or map[string]any.
func Iter[T any](ctx context.Context, b *SelectBuilder) iter.Seq2[T, error] {
	if b.exec == nil {
		return func(yield func(T, error) bool) {
			var zero T
			yield(zero, errors.New("corm: missing Executor for select"))
		}
	}
	rows, err := b.Query(ctx)
	if err != nil {
		return func(yield func(T, error) bool) {
			var zero T
			yield(zero, err)
		}
	}
	return scan.Iter[T](rows)
}

func (b *SelectBuilder) One(ctx context.Context, dest any) error {
	if b.exec == nil {
		return errors.New("corm: missing Executor for select")
	}
	rows, err := b.Query(ctx)
	if err != nil {
		return err
	}
	return scan.ScanOne(rows, dest)
}

func (b *SelectBuilder) Scalar(ctx context.Context, dest any) error {
	if b.exec == nil {
		return errors.New("corm: missing Executor for select")
	}
	rows, err := b.Query(ctx)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	return rows.Scan(dest)
}

// Count executes SELECT COUNT(*) and returns the count.
func (b *SelectBuilder) Count(ctx context.Context) (int64, error) {
	return b.CountExpr(ctx, countStarExpr)
}

// CountExpr executes SELECT <expr> and returns the count.
// Use this for custom count expressions like COUNT(DISTINCT column).
// When the original query has GROUP BY, it wraps the query in a subquery
// to return the total count of groups.
//
// Example:
//
//	count, err := query.CountExpr(ctx, clause.Raw("COUNT(DISTINCT `email`)"))
func (b *SelectBuilder) CountExpr(ctx context.Context, expr clause.Expr) (int64, error) {
	if b.exec == nil {
		return 0, errors.New("corm: missing Executor for select")
	}
	if len(b.groupBy) > 0 {
		wrapped := &SelectBuilder{
			exec:      b.exec,
			d:         b.d,
			columns:   []selectColumnItem{{kind: selectColumnExpr, expr: expr}},
			fromSub:   b,
			fromAlias: "sub",
			err:       b.err,
		}
		var count int64
		if err := wrapped.Scalar(ctx, &count); err != nil {
			return 0, err
		}
		return count, nil
	}
	cb := &SelectBuilder{
		exec:      b.exec,
		d:         b.d,
		columns:   []selectColumnItem{{kind: selectColumnExpr, expr: expr}},
		fromTable: b.fromTable,
		fromSub:   b.fromSub,
		fromAlias: b.fromAlias,
		joins:     b.joins,
		where:     b.where,
		err:       b.err,
	}
	var count int64
	if err := cb.Scalar(ctx, &count); err != nil {
		return 0, err
	}
	return count, nil
}

// CountExprSQL returns the SQL string and args for a count query with a custom expression.
// This is useful for debugging and testing without executing the query.
func (b *SelectBuilder) CountExprSQL(expr clause.Expr) (string, []any, error) {
	if len(b.groupBy) > 0 {
		wrapped := &SelectBuilder{
			d:         b.d,
			columns:   []selectColumnItem{{kind: selectColumnExpr, expr: expr}},
			fromSub:   b,
			fromAlias: "sub",
			err:       b.err,
		}
		return wrapped.SQL()
	}
	cb := &SelectBuilder{
		d:         b.d,
		columns:   []selectColumnItem{{kind: selectColumnExpr, expr: expr}},
		fromTable: b.fromTable,
		fromSub:   b.fromSub,
		fromAlias: b.fromAlias,
		joins:     b.joins,
		where:     b.where,
		err:       b.err,
	}
	return cb.SQL()
}

// Exists executes SELECT EXISTS(subquery) and returns true if rows exist.
func (b *SelectBuilder) Exists(ctx context.Context) (bool, error) {
	if b.exec == nil {
		return false, errors.New("corm: missing Executor for select")
	}
	if b.err != nil {
		return false, b.err
	}
	if b.d == nil {
		return false, errors.New("corm: nil dialect")
	}

	// Build EXISTS(SELECT 1 FROM ... WHERE ...)
	inner := &SelectBuilder{
		exec:      b.exec,
		d:         b.d,
		columns:   []selectColumnItem{{kind: selectColumnExpr, expr: oneExpr}},
		fromTable: b.fromTable,
		fromSub:   b.fromSub,
		fromAlias: b.fromAlias,
		joins:     b.joins,
		where:     b.where,
		limit:     new(1),
		err:       b.err,
	}

	sqlStr, args, err := inner.SQL()
	if err != nil {
		return false, err
	}
	existsSQL := "SELECT EXISTS(" + sqlStr + ")"
	rows, err := b.exec.QueryContext(ctx, existsSQL, args...)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return false, err
		}
		return false, nil
	}
	var exists bool
	if err := rows.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

package builder

import (
	"context"
	"database/sql"
	"errors"

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
	if b.exec == nil {
		return 0, errors.New("corm: missing Executor for select")
	}
	// Build a count query by replacing columns
	cb := &SelectBuilder{
		exec:      b.exec,
		d:         b.d,
		columns:   []selectColumnItem{{kind: selectColumnExpr, expr: countStarExpr}},
		fromTable: b.fromTable,
		fromSub:   b.fromSub,
		fromAlias: b.fromAlias,
		joins:     b.joins,
		where:     b.where,
		groupBy:   b.groupBy,
		having:    b.having,
		err:       b.err,
	}
	var count int64
	if err := cb.Scalar(ctx, &count); err != nil {
		return 0, err
	}
	return count, nil
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
		limit:     intPtr(1),
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

func intPtr(v int) *int { return &v }

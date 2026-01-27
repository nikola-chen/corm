package builder

import (
	"context"
	"database/sql"

	"github.com/nikola-chen/corm/exec"
)

func (b *SelectBuilder) All(ctx context.Context, dest any) error {
	rows, err := b.Query(ctx)
	if err != nil {
		return err
	}
	return exec.ScanAll(rows, dest)
}

func (b *SelectBuilder) One(ctx context.Context, dest any) error {
	rows, err := b.Query(ctx)
	if err != nil {
		return err
	}
	return exec.ScanOne(rows, dest)
}

func (b *SelectBuilder) Scalar(ctx context.Context, dest any) error {
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

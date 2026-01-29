package builder

import (
	"context"
	"database/sql"

	"github.com/nikola-chen/corm/scan"
)

func (b *SelectBuilder) All(ctx context.Context, dest any) error {
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
	rows, err := b.Query(ctx)
	if err != nil {
		return err
	}
	return scan.ScanOne(rows, dest)
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

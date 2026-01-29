package builder

import (
	"context"
	"errors"

	"github.com/nikola-chen/corm/scan"
)

func (b *InsertBuilder) One(ctx context.Context, dest any) error {
	if b.err != nil {
		return b.err
	}
	if b.exec == nil {
		return errors.New("corm: missing Executor for insert")
	}
	if len(b.returning) == 0 {
		return errors.New("corm: insert.One requires Returning(...)")
	}
	if !b.d.SupportsReturning() {
		return errors.New("corm: dialect does not support returning")
	}
	sqlStr, args, err := b.SQL()
	if err != nil {
		return err
	}
	rows, err := b.exec.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return err
	}
	return scan.ScanOne(rows, dest)
}

package builder

import (
	"context"

	"github.com/nikola-chen/corm/scan"
)

func (b *InsertBuilder) One(ctx context.Context, dest any) error {
	if b.err != nil {
		return b.err
	}
	if b.exec == nil {
		return errMissingInsertExec
	}
	if len(b.returning) == 0 {
		return errInsertOneNoReturning
	}
	if !b.d.SupportsReturning() {
		return errInsertOneDialect
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

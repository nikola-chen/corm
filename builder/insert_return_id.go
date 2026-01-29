package builder

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
)

func (b *InsertBuilder) ExecAndReturnID(ctx context.Context, idColumn string) (int64, error) {
	var id int64
	if err := b.ExecAndReturnIDInto(ctx, idColumn, &id); err != nil {
		return 0, err
	}
	return id, nil
}

func (b *InsertBuilder) ExecAndReturnIDInto(ctx context.Context, idColumn string, dest any) error {
	if b.err != nil {
		return b.err
	}
	if b.exec == nil {
		return errors.New("corm: missing Executor for insert")
	}
	idColumn = strings.TrimSpace(idColumn)
	if idColumn == "" {
		idColumn = "id"
	}
	if b.fromSelect != nil || len(b.rows) != 1 {
		return errors.New("corm: insert returning id requires single-row insert")
	}

	if b.d.SupportsReturning() {
		_, ok := quoteColumnStrict(b.d, idColumn)
		if !ok {
			return errors.New("corm: invalid column identifier")
		}
		prevReturning := b.returning
		b.returning = []string{idColumn}
		defer func() { b.returning = prevReturning }()

		sqlStr, args, err := b.SQL()
		if err != nil {
			return err
		}
		rows, err := b.exec.QueryContext(ctx, sqlStr, args...)
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

	res, err := b.Exec(ctx)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	return setInt64(dest, id)
}

func setInt64(dest any, v int64) error {
	switch p := dest.(type) {
	case *int:
		*p = int(v)
		return nil
	case *int64:
		*p = v
		return nil
	case *uint:
		*p = uint(v)
		return nil
	case *uint64:
		*p = uint64(v)
		return nil
	case *sql.NullInt64:
		p.Int64 = v
		p.Valid = true
		return nil
	}

	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.New("corm: dest must be non-nil pointer")
	}
	ev := rv.Elem()
	switch ev.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if ev.OverflowInt(v) {
			return errors.New("corm: dest int overflow")
		}
		ev.SetInt(v)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if v < 0 || ev.OverflowUint(uint64(v)) {
			return errors.New("corm: dest uint overflow")
		}
		ev.SetUint(uint64(v))
		return nil
	default:
		return errors.New("corm: dest must be integer pointer")
	}
}

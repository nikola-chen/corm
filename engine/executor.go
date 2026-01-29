package engine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nikola-chen/corm/builder"
)

type loggingExecutor struct {
	inner  builder.Executor
	logger Logger
	cfg    Config
}

func (l *loggingExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	res, err := l.inner.ExecContext(ctx, query, args...)
	l.log(query, args, time.Since(start), err)
	return res, err
}

func (l *loggingExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := l.inner.QueryContext(ctx, query, args...)
	l.log(query, args, time.Since(start), err)
	return rows, err
}

func (l *loggingExecutor) log(query string, args []any, dur time.Duration, err error) {
	if l.logger == nil {
		return
	}
	if !l.cfg.LogSQL && (l.cfg.SlowQuery <= 0 || dur < l.cfg.SlowQuery) {
		return
	}
	if !l.cfg.LogCanceled && err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			err = errors.New("corm: context canceled")
		}
	}
	query = truncateSQL(query, l.cfg.MaxLogSQLLen)
	if l.cfg.LogArgs {
		l.logger.Printf("sql=%s args=%s dur=%s err=%v", query, formatArgs(args, l.cfg.ArgFormatter, l.cfg.MaxLogArgsItems, l.cfg.MaxLogArgsLen), dur, err)
		return
	}
	l.logger.Printf("sql=%s argc=%d dur=%s err=%v", query, len(args), dur, err)
}

func truncateSQL(sql string, maxLen int) string {
	const defaultMax = 2048
	if maxLen <= 0 {
		maxLen = defaultMax
	}
	if len(sql) <= maxLen {
		return sql
	}
	return sql[:maxLen] + "…"
}

func formatArgs(args []any, argFormatter func(any) string, maxItems int, maxLen int) string {
	const defaultMaxItems = 20
	const defaultMaxLen = 512
	if maxItems <= 0 {
		maxItems = defaultMaxItems
	}
	if maxLen <= 0 {
		maxLen = defaultMaxLen
	}
	if argFormatter == nil {
		argFormatter = defaultArgFormatter
	}
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < len(args) && i < maxItems; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(argFormatter(args[i]))
		if b.Len() > maxLen {
			b.WriteString("…")
			break
		}
	}
	if len(args) > maxItems {
		if b.Len() > 1 {
			b.WriteString(", ")
		}
		b.WriteString("…")
	}
	b.WriteByte(']')
	return b.String()
}

func defaultArgFormatter(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return fmt.Sprintf("redacted(len=%d)", len(x))
	case []byte:
		return fmt.Sprintf("bytes(len=%d)", len(x))
	case error:
		return fmt.Sprintf("%T(redacted)", v)
	case fmt.Stringer:
		return fmt.Sprintf("%T(redacted)", v)
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprintf("%v", v)
	default:
		s := fmt.Sprintf("%v", v)
		if len(s) > 64 {
			return s[:64] + "…"
		}
		return s
	}
}

func (e *Engine) executor() builder.Executor {
	if e.logger == nil {
		return e.db
	}
	if !e.cfg.LogSQL && e.cfg.SlowQuery <= 0 {
		return e.db
	}
	return &loggingExecutor{inner: e.db, logger: e.logger, cfg: e.cfg}
}

func (t *Tx) executor() builder.Executor {
	if t.logger == nil {
		return t.tx
	}
	if !t.cfg.LogSQL && t.cfg.SlowQuery <= 0 {
		return t.tx
	}
	return &loggingExecutor{inner: t.tx, logger: t.logger, cfg: t.cfg}
}

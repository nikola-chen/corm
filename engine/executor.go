package engine

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type coreExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type loggingExecutor struct {
	inner  coreExecutor
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

func (l *loggingExecutor) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return l.inner.QueryRowContext(ctx, query, args...)
}

func (l *loggingExecutor) log(query string, args []any, dur time.Duration, err error) {
	if l.logger == nil {
		return
	}
	if !l.cfg.LogSQL && (l.cfg.SlowQuery <= 0 || dur < l.cfg.SlowQuery) {
		return
	}
	if l.cfg.LogArgs {
		l.logger.Printf("sql=%s args=%s dur=%s err=%v", query, formatArgs(args), dur, err)
		return
	}
	l.logger.Printf("sql=%s argc=%d dur=%s err=%v", query, len(args), dur, err)
}

func formatArgs(args []any) string {
	const maxItems = 20
	const maxLen = 512
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < len(args) && i < maxItems; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(fmt.Sprintf("%v", args[i]))
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

func (e *Engine) executor() coreExecutor {
	if e.logger == nil {
		return e.db
	}
	if !e.cfg.LogSQL && e.cfg.SlowQuery <= 0 {
		return e.db
	}
	return &loggingExecutor{inner: e.db, logger: e.logger, cfg: e.cfg}
}

func (t *Tx) executor() coreExecutor {
	if t.logger == nil {
		return t.tx
	}
	if !t.cfg.LogSQL && t.cfg.SlowQuery <= 0 {
		return t.tx
	}
	return &loggingExecutor{inner: t.tx, logger: t.logger, cfg: t.cfg}
}


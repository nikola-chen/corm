package engine

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/nikola-chen/corm/builder"
	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/dialect"
)

// Logger interface for logging SQL and errors.
type Logger interface {
	Printf(format string, args ...any)
}

// Config defines the configuration for Engine.
type Config struct {
	// MaxOpenConns sets the maximum number of open connections to the database.
	MaxOpenConns int
	// MaxIdleConns sets the maximum number of connections in the idle connection pool.
	MaxIdleConns int
	// ConnMaxLifetime sets the maximum amount of time a connection may be reused.
	ConnMaxLifetime time.Duration
	// LogSQL enables SQL logging.
	LogSQL bool
	// LogArgs enables argument logging in SQL logs.
	LogArgs bool
	// ArgFormatter formats each argument when LogArgs is enabled.
	// If nil, a safe default formatter is used.
	ArgFormatter func(any) string
	// MaxLogSQLLen truncates SQL in logs to at most this many bytes.
	// If 0, a safe default is used.
	MaxLogSQLLen int
	// MaxLogArgsItems limits how many args are rendered when LogArgs is enabled.
	// If 0, a safe default is used.
	MaxLogArgsItems int
	// MaxLogArgsLen limits the total args string length when LogArgs is enabled.
	// If 0, a safe default is used.
	MaxLogArgsLen int
	// LogCanceled controls whether context canceled/deadline errors are logged verbosely.
	LogCanceled bool
	// SlowQuery sets the threshold for slow query logging.
	SlowQuery time.Duration
}

// Option is a function to configure the Engine.
type Option func(*Engine) error

// WithLogger sets the logger for the Engine.
func WithLogger(logger Logger) Option {
	return func(e *Engine) error {
		e.logger = logger
		return nil
	}
}

// WithConfig sets the configuration for the Engine.
func WithConfig(cfg Config) Option {
	return func(e *Engine) error {
		e.cfg = cfg
		return nil
	}
}

// Engine is the main entry point for the ORM.
type Engine struct {
	db      *sql.DB
	dialect dialect.Dialect
	logger  Logger
	cfg     Config
}

// Open opens a database connection.
func Open(driverName, dsn string, opts ...Option) (*Engine, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}
	return WithDB(db, driverName, opts...)
}

// WithDB creates an Engine with an existing sql.DB.
func WithDB(db *sql.DB, driverName string, opts ...Option) (*Engine, error) {
	d, ok := dialect.Get(driverName)
	if !ok {
		return nil, errors.New("corm: unsupported dialect: " + driverName)
	}

	e := &Engine{
		db:      db,
		dialect: d,
		logger:  NopLogger{},
	}
	for _, opt := range opts {
		if err := opt(e); err != nil {
			return nil, err
		}
	}

	if e.cfg.MaxOpenConns > 0 {
		e.db.SetMaxOpenConns(e.cfg.MaxOpenConns)
	}
	if e.cfg.MaxIdleConns > 0 {
		e.db.SetMaxIdleConns(e.cfg.MaxIdleConns)
	}
	if e.cfg.ConnMaxLifetime > 0 {
		e.db.SetConnMaxLifetime(e.cfg.ConnMaxLifetime)
	}

	return e, nil
}

// DB returns the underlying sql.DB.
func (e *Engine) DB() *sql.DB {
	return e.db
}

// Dialect returns the database dialect.
func (e *Engine) Dialect() dialect.Dialect {
	return e.dialect
}

// Close closes the database connection.
func (e *Engine) Close() error {
	return e.db.Close()
}

// Stats returns database statistics.
func (e *Engine) Stats() sql.DBStats {
	return e.db.Stats()
}

// Ping verifies a connection to the database is still alive.
func (e *Engine) Ping(ctx context.Context) error {
	return e.db.PingContext(ctx)
}

// Transaction executes the given function within a transaction.
// It automatically commits the transaction if the function returns nil,
// and rolls back if the function returns an error or panics.
func (e *Engine) Transaction(ctx context.Context, fn func(*Tx) error) (err error) {
	tx, err := e.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(); rbErr != nil && e.logger != nil {
				e.logger.Printf("corm: rollback failed err=%v", rbErr)
			}
			panic(p) // re-throw panic after rollback
		} else if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil && e.logger != nil {
				e.logger.Printf("corm: rollback failed err=%v", rbErr)
			}
		} else {
			err = tx.Commit()
		}
	}()

	err = fn(tx)
	return err
}

// Select creates a new SelectBuilder.
func (e *Engine) Select(columns ...string) *builder.SelectBuilder {
	return builder.Select(e.executor(), e.dialect, columns...)
}

// SelectExpr creates a new SelectBuilder with expression columns.
func (e *Engine) SelectExpr(columns ...clause.Expr) *builder.SelectBuilder {
	return builder.Select(e.executor(), e.dialect).SelectExpr(columns...)
}

// Insert creates a new InsertBuilder.
func (e *Engine) Insert(table string) *builder.InsertBuilder {
	return builder.Insert(e.executor(), e.dialect, table)
}

// Update creates a new UpdateBuilder.
func (e *Engine) Update(table string) *builder.UpdateBuilder {
	return builder.Update(e.executor(), e.dialect, table)
}

// Delete creates a new DeleteBuilder.
func (e *Engine) Delete(table string) *builder.DeleteBuilder {
	return builder.Delete(e.executor(), e.dialect, table)
}

// Builder returns a builder.API that is pre-bound to this Engine's dialect and executor.
// It is useful when you want to pass a lightweight query builder handle around without
// repeating dialect/executor wiring.
func (e *Engine) Builder() *builder.API {
	return builder.NewAPI(e.dialect, e.executor())
}

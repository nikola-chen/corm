package engine

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"corm/builder"
	"corm/dialect"
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
			_ = tx.Rollback()
			panic(p) // re-throw panic after rollback
		} else if err != nil {
			_ = tx.Rollback()
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

// SelectColumns creates a new SelectBuilder with a slice of columns.
func (e *Engine) SelectColumns(columns []string) *builder.SelectBuilder {
	return builder.Select(e.executor(), e.dialect, columns...)
}

// InsertInto creates a new InsertBuilder.
func (e *Engine) InsertInto(table string) *builder.InsertBuilder {
	return builder.InsertInto(e.executor(), e.dialect, table)
}

// Update creates a new UpdateBuilder.
func (e *Engine) Update(table string) *builder.UpdateBuilder {
	return builder.Update(e.executor(), e.dialect, table)
}

// DeleteFrom creates a new DeleteBuilder.
func (e *Engine) DeleteFrom(table string) *builder.DeleteBuilder {
	return builder.DeleteFrom(e.executor(), e.dialect, table)
}

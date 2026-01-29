package corm

import (
	"database/sql"

	"github.com/nikola-chen/corm/engine"
)

type DB = engine.Engine
type Tx = engine.Tx
type Logger = engine.Logger
type Config = engine.Config
type Option = engine.Option

func Open(driverName, dsn string, opts ...Option) (*DB, error) {
	return engine.Open(driverName, dsn, opts...)
}

func WithDB(db *sql.DB, driverName string, opts ...Option) (*DB, error) {
	return engine.WithDB(db, driverName, opts...)
}

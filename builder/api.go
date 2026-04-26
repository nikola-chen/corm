package builder

import (
	"errors"
	"strings"

	"github.com/nikola-chen/corm/dialect"
)

// API pre-binds a dialect and an optional Executor to reduce repetitive arguments
// when using the builder package directly.
type API struct {
	d    dialect.Dialect
	exec Executor
	err  error
}

// NewAPI creates an API bound to a dialect and an optional Executor.
//
// If d is nil, the returned API is still usable for chaining, but any builder
// produced from it will return an error from SQL()/Exec()/Query().
func NewAPI(d dialect.Dialect, exec Executor) *API {
	if d == nil {
		return &API{exec: exec, err: errors.New("corm: nil dialect")}
	}
	return &API{d: d, exec: exec}
}

// New is a shortcut for NewAPI(d, nil). It is useful when you only build SQL strings.
func New(d dialect.Dialect) *API {
	return NewAPI(d, nil)
}

// Dialect creates an API by driver name (e.g. "mysql", "postgres").
//
// If the dialect is not registered, the returned API will carry an error which
// will be propagated to generated builders.
func Dialect(driver string) *API {
	driver = strings.TrimSpace(driver)
	if driver == "" {
		return &API{err: errors.New("corm: empty dialect")}
	}
	d, ok := dialect.Get(driver)
	if !ok || d == nil {
		return &API{err: errors.New("corm: unsupported dialect: " + driver)}
	}
	return &API{d: d}
}

// MySQL returns an API bound to the MySQL dialect.
func MySQL() *API {
	return Dialect("mysql")
}

// Postgres returns an API bound to the PostgreSQL dialect.
func Postgres() *API {
	return Dialect("postgres")
}

// For creates an API bound to the given dialect and Executor.
// This is a convenience wrapper around NewAPI for the common case
// of binding both a dialect and an executor.
func For(d dialect.Dialect, exec Executor) *API {
	return NewAPI(d, exec)
}

// MustFor is like For but panics if the dialect is nil.
func MustFor(d dialect.Dialect, exec Executor) *API {
	if d == nil {
		panic("corm: nil dialect")
	}
	return NewAPI(d, exec)
}

// MustDialect is like Dialect but panics if the dialect is not registered.
func MustDialect(driver string) *API {
	driver = strings.TrimSpace(driver)
	if driver == "" {
		panic("corm: empty dialect")
	}
	d := dialect.MustGet(driver)
	return &API{d: d}
}

// Dialect returns the dialect bound to this API, or nil if none.
func (a *API) Dialect() dialect.Dialect {
	if a == nil {
		return nil
	}
	return a.d
}

// Err returns any error stored in the API, or nil.
func (a *API) Err() error {
	if a == nil {
		return errors.New("corm: nil api")
	}
	return a.err
}

// Select creates a SelectBuilder using the API's dialect and Executor.
func (a *API) Select(columns ...string) *SelectBuilder {
	if a == nil {
		return &SelectBuilder{err: errors.New("corm: nil api")}
	}
	b := newSelectBuilder(a.exec, a.d, columns...)
	if a.err != nil {
		b.err = a.err
		return b
	}
	if a.d == nil {
		b.err = errors.New("corm: nil dialect")
	}
	return b
}

// Insert creates an InsertBuilder using the API's dialect and Executor.
func (a *API) Insert(table string) *InsertBuilder {
	if a == nil {
		return &InsertBuilder{err: errors.New("corm: nil api")}
	}
	b := newInsertBuilder(a.exec, a.d, table)
	if a.err != nil {
		b.err = a.err
		return b
	}
	if a.d == nil {
		b.err = errors.New("corm: nil dialect")
	}
	return b
}

// Update creates an UpdateBuilder using the API's dialect and Executor.
func (a *API) Update(table string) *UpdateBuilder {
	if a == nil {
		return &UpdateBuilder{err: errors.New("corm: nil api")}
	}
	b := newUpdateBuilder(a.exec, a.d, table)
	if a.err != nil {
		b.err = a.err
		return b
	}
	if a.d == nil {
		b.err = errors.New("corm: nil dialect")
	}
	return b
}

// Delete creates a DeleteBuilder using the API's dialect and Executor.
func (a *API) Delete(table string) *DeleteBuilder {
	if a == nil {
		return &DeleteBuilder{err: errors.New("corm: nil api")}
	}
	b := newDeleteBuilder(a.exec, a.d, table)
	if a.err != nil {
		b.err = a.err
		return b
	}
	if a.d == nil {
		b.err = errors.New("corm: nil dialect")
	}
	return b
}

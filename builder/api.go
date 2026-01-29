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

// MustDialect creates an API by driver name or panics if the dialect is not registered.
func MustDialect(driver string) *API {
	a := Dialect(driver)
	if a.err != nil {
		panic(a.err)
	}
	return a
}

// For creates an API by driver name and binds an Executor.
func For(driver string, exec Executor) *API {
	a := Dialect(driver)
	a.exec = exec
	return a
}

// MustFor creates an API by driver name and binds an Executor or panics if unsupported.
func MustFor(driver string, exec Executor) *API {
	a := For(driver, exec)
	if a.err != nil {
		panic(a.err)
	}
	return a
}

// MySQL returns an API bound to the MySQL dialect.
func MySQL() *API {
	return Dialect("mysql")
}

// Postgres returns an API bound to the PostgreSQL dialect.
func Postgres() *API {
	return Dialect("postgres")
}

// Select creates a SelectBuilder using the API's dialect and Executor.
func (a *API) Select(columns ...string) *SelectBuilder {
	if a == nil {
		return &SelectBuilder{err: errors.New("corm: nil api")}
	}
	b := Select(a.exec, a.d, columns...)
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
	b := Insert(a.exec, a.d, table)
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
	b := Update(a.exec, a.d, table)
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
	b := Delete(a.exec, a.d, table)
	if a.err != nil {
		b.err = a.err
		return b
	}
	if a.d == nil {
		b.err = errors.New("corm: nil dialect")
	}
	return b
}

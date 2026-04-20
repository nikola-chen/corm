package builder

import "github.com/nikola-chen/corm/dialect"

// newBuilder creates builders with the given executor and dialect.
// These are low-level constructors; prefer using the API type for most use cases.
func newSelectBuilder(exec Executor, d dialect.Dialect, columns ...string) *SelectBuilder {
	return newSelect(exec, d, columns)
}

func newInsertBuilder(exec Executor, d dialect.Dialect, table string) *InsertBuilder {
	return newInsert(exec, d, table)
}

func newUpdateBuilder(exec Executor, d dialect.Dialect, table string) *UpdateBuilder {
	return newUpdate(exec, d, table)
}

func newDeleteBuilder(exec Executor, d dialect.Dialect, table string) *DeleteBuilder {
	return newDelete(exec, d, table)
}

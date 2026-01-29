package builder

import "github.com/nikola-chen/corm/dialect"

func Select(exec Executor, d dialect.Dialect, columns ...string) *SelectBuilder {
	return newSelect(exec, d, columns)
}

func Insert(exec Executor, d dialect.Dialect, table string) *InsertBuilder {
	return newInsert(exec, d, table)
}

func Update(exec Executor, d dialect.Dialect, table string) *UpdateBuilder {
	return newUpdate(exec, d, table)
}

func Delete(exec Executor, d dialect.Dialect, table string) *DeleteBuilder {
	return newDelete(exec, d, table)
}

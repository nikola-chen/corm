package builder

import "github.com/nikola-chen/corm/dialect"

func Select(exec executor, d dialect.Dialect, columns ...string) *SelectBuilder {
	return newSelect(exec, d, columns)
}

func InsertInto(exec executor, d dialect.Dialect, table string) *InsertBuilder {
	return newInsert(exec, d, table)
}

func Update(exec executor, d dialect.Dialect, table string) *UpdateBuilder {
	return newUpdate(exec, d, table)
}

func DeleteFrom(exec executor, d dialect.Dialect, table string) *DeleteBuilder {
	return newDelete(exec, d, table)
}


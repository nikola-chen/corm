package builder_test

import "github.com/nikola-chen/corm/builder"

func mysqlQB() *builder.API {
	return builder.Dialect("mysql")
}

func pgQB() *builder.API {
	return builder.Dialect("postgres")
}

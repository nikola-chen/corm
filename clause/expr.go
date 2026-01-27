package clause

import (
	"reflect"
	"strings"
)

// Expr represents a SQL expression fragment.
type Expr struct {
	SQL  string
	Args []any
}

// Raw creates a raw SQL expression.
func Raw(sql string, args ...any) Expr {
	return Expr{SQL: sql, Args: args}
}

// And joins multiple expressions with AND.
func And(exprs ...Expr) Expr {
	return join("AND", exprs...)
}

// Or joins multiple expressions with OR.
func Or(exprs ...Expr) Expr {
	return join("OR", exprs...)
}

// In creates an IN expression: "column IN (?, ?, ...)"
// It automatically flattens slice arguments.
func In(column string, values ...any) Expr {
	flattened := make([]any, 0, len(values))
	for _, v := range values {
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() != reflect.Uint8 { // exclude []byte
			for i := 0; i < rv.Len(); i++ {
				flattened = append(flattened, rv.Index(i).Interface())
			}
		} else {
			flattened = append(flattened, v)
		}
	}

	if len(flattened) == 0 {
		// "IN ()" is usually invalid or false. 
		// We return "1=0" to ensure it matches nothing.
		return Expr{SQL: "1=0"}
	}

	placeholders := make([]string, len(flattened))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	sql := column + " IN (" + strings.Join(placeholders, ", ") + ")"
	return Expr{SQL: sql, Args: flattened}
}

func join(op string, exprs ...Expr) Expr {
	var parts []string
	args := make([]any, 0)
	for _, e := range exprs {
		if strings.TrimSpace(e.SQL) == "" {
			continue
		}
		parts = append(parts, "("+e.SQL+")")
		args = append(args, e.Args...)
	}
	if len(parts) == 0 {
		return Expr{}
	}
	return Expr{SQL: strings.Join(parts, " "+op+" "), Args: args}
}


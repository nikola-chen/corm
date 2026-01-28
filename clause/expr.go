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

// Eq creates an equality expression: "column = value"
func Eq(column string, value any) Expr {
	return Expr{SQL: column + " = ?", Args: []any{value}}
}

// Neq creates a non-equality expression: "column != value"
func Neq(column string, value any) Expr {
	return Expr{SQL: column + " != ?", Args: []any{value}}
}

// Gt creates a greater than expression: "column > value"
func Gt(column string, value any) Expr {
	return Expr{SQL: column + " > ?", Args: []any{value}}
}

// Gte creates a greater than or equal expression: "column >= value"
func Gte(column string, value any) Expr {
	return Expr{SQL: column + " >= ?", Args: []any{value}}
}

// Lt creates a less than expression: "column < value"
func Lt(column string, value any) Expr {
	return Expr{SQL: column + " < ?", Args: []any{value}}
}

// Lte creates a less than or equal expression: "column <= value"
func Lte(column string, value any) Expr {
	return Expr{SQL: column + " <= ?", Args: []any{value}}
}

// Like creates a LIKE expression: "column LIKE value"
func Like(column string, value any) Expr {
	return Expr{SQL: column + " LIKE ?", Args: []any{value}}
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

// Not negates an expression: "NOT (expr)"
func Not(expr Expr) Expr {
	if strings.TrimSpace(expr.SQL) == "" {
		return Expr{}
	}
	return Expr{SQL: "NOT (" + expr.SQL + ")", Args: expr.Args}
}

// IsNull checks if a column is NULL.
func IsNull(column string) Expr {
	return Expr{SQL: column + " IS NULL"}
}

// IsNotNull checks if a column is NOT NULL.
func IsNotNull(column string) Expr {
	return Expr{SQL: column + " IS NOT NULL"}
}

// Alias creates an alias expression: "expression AS alias"
func Alias(expression, alias string) string {
	return expression + " AS " + alias
}

// Any creates an ANY expression: "column operator ANY (subquery)"
// This is a placeholder; actual usage often involves subqueries.
func Any(column, operator, subquery string) Expr {
	return Expr{SQL: column + " " + operator + " ANY (" + subquery + ")"}
}

// All creates an ALL expression: "column operator ALL (subquery)"
func All(column, operator, subquery string) Expr {
	return Expr{SQL: column + " " + operator + " ALL (" + subquery + ")"}
}

// Some is an alias for Any.
func Some(column, operator, subquery string) Expr {
	return Any(column, operator, subquery)
}

// Count creates a COUNT expression.
func Count(column string) string {
	return "COUNT(" + column + ")"
}

// Sum creates a SUM expression.
func Sum(column string) string {
	return "SUM(" + column + ")"
}

// Avg creates an AVG expression.
func Avg(column string) string {
	return "AVG(" + column + ")"
}

// Max creates a MAX expression.
func Max(column string) string {
	return "MAX(" + column + ")"
}

// Min creates a MIN expression.
func Min(column string) string {
	return "MIN(" + column + ")"
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


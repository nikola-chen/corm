package clause

import (
	"reflect"
	"strings"
)

var emptyArgs = make([]any, 0)

// EmptyArgs returns an empty args slice for reuse.
func EmptyArgs() []any {
	return emptyArgs
}

// Expr represents a SQL expression fragment.
type Expr struct {
	SQL  string
	Args []any
}

// Raw creates a raw SQL expression.
func Raw(sql string, args ...any) Expr {
	if len(args) == 0 {
		return Expr{SQL: sql, Args: emptyArgs}
	}
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

// Eq creates an equality expression: "column = ?".
// The column must be a trusted identifier (do not pass user input).
func Eq(column string, value any) Expr {
	return Expr{SQL: column + " = ?", Args: []any{value}}
}

// Neq creates a non-equality expression: "column != value"
// The column must be a trusted identifier (do not pass user input).
func Neq(column string, value any) Expr {
	return Expr{SQL: column + " != ?", Args: []any{value}}
}

// Gt creates a greater than expression: "column > value"
// The column must be a trusted identifier (do not pass user input).
func Gt(column string, value any) Expr {
	return Expr{SQL: column + " > ?", Args: []any{value}}
}

// Gte creates a greater than or equal expression: "column >= value"
// The column must be a trusted identifier (do not pass user input).
func Gte(column string, value any) Expr {
	return Expr{SQL: column + " >= ?", Args: []any{value}}
}

// Lt creates a less than expression: "column < value"
// The column must be a trusted identifier (do not pass user input).
func Lt(column string, value any) Expr {
	return Expr{SQL: column + " < ?", Args: []any{value}}
}

// Lte creates a less than or equal expression: "column <= value"
// The column must be a trusted identifier (do not pass user input).
func Lte(column string, value any) Expr {
	return Expr{SQL: column + " <= ?", Args: []any{value}}
}

// Like creates a LIKE expression: "column LIKE value"
// The column must be a trusted identifier (do not pass user input).
func Like(column string, value any) Expr {
	return Expr{SQL: column + " LIKE ?", Args: []any{value}}
}

// In creates an IN expression: "column IN (?, ?, ...)".
// The column must be a trusted identifier (do not pass user input).
// It automatically flattens slice arguments.
func In(column string, values ...any) Expr {
	if len(values) == 0 {
		return Expr{SQL: "1=0", Args: emptyArgs}
	}

	// Fast path for single slice argument
	if len(values) == 1 {
		switch s := values[0].(type) {
		case []any:
			if len(s) == 0 {
				return Expr{SQL: "1=0", Args: emptyArgs}
			}
			flattened := make([]any, len(s))
			copy(flattened, s)
			return buildIn(column, flattened)
		case []string:
			if len(s) == 0 {
				return Expr{SQL: "1=0", Args: emptyArgs}
			}
			flattened := make([]any, len(s))
			for i := range s {
				flattened[i] = s[i]
			}
			return buildIn(column, flattened)
		case []int:
			if len(s) == 0 {
				return Expr{SQL: "1=0", Args: emptyArgs}
			}
			flattened := make([]any, len(s))
			for i := range s {
				flattened[i] = s[i]
			}
			return buildIn(column, flattened)
		case []int64:
			if len(s) == 0 {
				return Expr{SQL: "1=0", Args: emptyArgs}
			}
			flattened := make([]any, len(s))
			for i := range s {
				flattened[i] = s[i]
			}
			return buildIn(column, flattened)
		case []uint64:
			if len(s) == 0 {
				return Expr{SQL: "1=0", Args: emptyArgs}
			}
			flattened := make([]any, len(s))
			for i := range s {
				flattened[i] = s[i]
			}
			return buildIn(column, flattened)
		case []int32:
			if len(s) == 0 {
				return Expr{SQL: "1=0", Args: emptyArgs}
			}
			flattened := make([]any, len(s))
			for i := range s {
				flattened[i] = s[i]
			}
			return buildIn(column, flattened)
		case []uint:
			if len(s) == 0 {
				return Expr{SQL: "1=0", Args: emptyArgs}
			}
			flattened := make([]any, len(s))
			for i := range s {
				flattened[i] = s[i]
			}
			return buildIn(column, flattened)
		case []byte:
		default:
			rv := reflect.ValueOf(values[0])
			if rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() != reflect.Uint8 {
				n := rv.Len()
				if n == 0 {
					return Expr{SQL: "1=0", Args: emptyArgs}
				}

				// Optimization: if it's a slice of simple types, we can iterate without reflection for each element?
				// But we are in 'default' case, so we don't know the type.
				// However, rv.Index(i).Interface() allocates.
				// We can try to handle more common types here if needed (e.g. []uint, []int32).
				// For now, the reflect path is acceptable for obscure types.

				flattened := make([]any, n)
				for i := 0; i < n; i++ {
					flattened[i] = rv.Index(i).Interface()
				}
				return buildIn(column, flattened)
			}
		}
	}

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
		return Expr{SQL: "1=0", Args: emptyArgs}
	}

	return buildIn(column, flattened)
}

func buildIn(column string, args []any) Expr {
	n := len(args)
	// Fast path for common small sizes
	switch n {
	case 1:
		return Expr{SQL: column + " IN (?)", Args: args}
	case 2:
		return Expr{SQL: column + " IN (?, ?)", Args: args}
	case 3:
		return Expr{SQL: column + " IN (?, ?, ?)", Args: args}
	case 4:
		return Expr{SQL: column + " IN (?, ?, ?, ?)", Args: args}
	case 5:
		return Expr{SQL: column + " IN (?, ?, ?, ?, ?)", Args: args}
	}

	var b strings.Builder
	// approximate size: column + " IN (" + (3*n) + ")"
	b.Grow(len(column) + 6 + n*3)

	b.WriteString(column)
	b.WriteString(" IN (")
	for i := range args {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteByte('?')
	}
	b.WriteByte(')')
	return Expr{SQL: b.String(), Args: args}
}

// Not negates an expression: "NOT (expr)"
func Not(expr Expr) Expr {
	if strings.TrimSpace(expr.SQL) == "" {
		return Expr{}
	}
	return Expr{SQL: "NOT (" + expr.SQL + ")", Args: expr.Args}
}

// IsNull checks if a column is NULL.
// The column must be a trusted identifier (do not pass user input).
func IsNull(column string) Expr {
	return Expr{SQL: column + " IS NULL", Args: emptyArgs}
}

// IsNotNull checks if a column is NOT NULL.
// The column must be a trusted identifier (do not pass user input).
func IsNotNull(column string) Expr {
	return Expr{SQL: column + " IS NOT NULL", Args: emptyArgs}
}

// Alias creates an alias expression: "expression AS alias"
// Both expression and alias must be trusted identifiers (do not pass user input).
func Alias(expression, alias string) string {
	return expression + " AS " + alias
}

// Any creates an ANY expression: "column operator ANY (subquery)"
// This is a placeholder; actual usage often involves subqueries.
// The column, operator and subquery must be trusted identifiers/SQL fragments.
func Any(column, operator, subquery string) Expr {
	return Expr{SQL: column + " " + operator + " ANY (" + subquery + ")", Args: emptyArgs}
}

// All creates an ALL expression: "column operator ALL (subquery)"
// The column, operator and subquery must be trusted identifiers/SQL fragments.
func All(column, operator, subquery string) Expr {
	return Expr{SQL: column + " " + operator + " ALL (" + subquery + ")", Args: emptyArgs}
}

// Some is an alias for Any.
// The column, operator and subquery must be trusted identifiers/SQL fragments.
func Some(column, operator, subquery string) Expr {
	return Any(column, operator, subquery)
}

// Count creates a COUNT expression.
// The column must be a trusted identifier (do not pass user input).
func Count(column string) Expr {
	return Expr{SQL: "COUNT(" + column + ")", Args: emptyArgs}
}

// Sum creates a SUM expression.
// The column must be a trusted identifier (do not pass user input).
func Sum(column string) Expr {
	return Expr{SQL: "SUM(" + column + ")", Args: emptyArgs}
}

// Avg creates an AVG expression.
// The column must be a trusted identifier (do not pass user input).
func Avg(column string) Expr {
	return Expr{SQL: "AVG(" + column + ")", Args: emptyArgs}
}

// Max creates a MAX expression.
// The column must be a trusted identifier (do not pass user input).
func Max(column string) Expr {
	return Expr{SQL: "MAX(" + column + ")", Args: emptyArgs}
}

// Min creates a MIN expression.
// The column must be a trusted identifier (do not pass user input).
func Min(column string) Expr {
	return Expr{SQL: "MIN(" + column + ")", Args: emptyArgs}
}

func join(op string, exprs ...Expr) Expr {
	if len(exprs) == 0 {
		return Expr{}
	}

	partsCap := 0
	argsCap := 0
	for _, e := range exprs {
		if strings.TrimSpace(e.SQL) == "" {
			continue
		}
		partsCap++
		argsCap += len(e.Args)
	}

	if partsCap == 0 {
		return Expr{}
	}

	parts := make([]string, 0, partsCap)
	args := make([]any, 0, argsCap)
	for _, e := range exprs {
		if strings.TrimSpace(e.SQL) == "" {
			continue
		}
		parts = append(parts, "("+e.SQL+")")
		args = append(args, e.Args...)
	}
	return Expr{SQL: strings.Join(parts, " "+op+" "), Args: args}
}

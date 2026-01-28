package builder

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/dialect"
)

// SelectBuilder builds SELECT statements.
type SelectBuilder struct {
	exec    executor
	d       dialect.Dialect
	columns []string
	table   string
	joins   []clause.Expr
	where   []clause.Expr
	groupBy []string
	having  []clause.Expr
	orderBy []string
	limit   *int
	offset  *int
	distinct bool
	unions   []clause.Expr
	tableArgs []any
	err error
}

func newSelect(exec executor, d dialect.Dialect, columns []string) *SelectBuilder {
	return &SelectBuilder{exec: exec, d: d, columns: columns}
}

// Distinct enables DISTINCT selection.
func (b *SelectBuilder) Distinct() *SelectBuilder {
	b.distinct = true
	return b
}

// Top is an alias for Limit.
func (b *SelectBuilder) Top(n int) *SelectBuilder {
	return b.Limit(n)
}

// From sets the table to select from.
func (b *SelectBuilder) From(table string) *SelectBuilder {
	b.table = table
	b.tableArgs = nil
	return b
}

func (b *SelectBuilder) FromIdent(table string) *SelectBuilder {
	if b.err != nil {
		return b
	}
	q, ok := quoteIdentStrict(b.d, table)
	if !ok {
		b.err = errors.New("corm: invalid table identifier")
		return b
	}
	b.table = q
	b.tableArgs = nil
	return b
}

func (b *SelectBuilder) FromAs(table, alias string) *SelectBuilder {
	if b.err != nil {
		return b
	}
	qTable, ok := quoteIdentStrict(b.d, table)
	if !ok {
		b.err = errors.New("corm: invalid table identifier")
		return b
	}
	alias = strings.TrimSpace(alias)
	if !isSimpleIdent(alias) {
		b.err = errors.New("corm: invalid alias identifier")
		return b
	}
	b.table = qTable + " AS " + alias
	b.tableArgs = nil
	return b
}

// FromSelect sets a subquery as the table to select from.
func (b *SelectBuilder) FromSelect(sub *SelectBuilder, alias string) *SelectBuilder {
	if b.err != nil {
		return b
	}
	sqlStr, args, err := sub.sqlRaw()
	if err != nil {
		b.err = err
		return b
	}
	alias = strings.TrimSpace(alias)
	if !isSimpleIdent(alias) {
		b.err = errors.New("corm: invalid alias identifier")
		return b
	}
	b.table = "(" + sqlStr + ") AS " + alias
	b.tableArgs = args
	return b
}

// Where adds a WHERE condition.
// It supports "id = ?", 1 or "name = ?", "alice".
// Do not concatenate untrusted user input into the SQL string; use args for parameter binding.
func (b *SelectBuilder) Where(sql string, args ...any) *SelectBuilder {
	b.where = append(b.where, clause.Raw(sql, args...))
	return b
}

func (b *SelectBuilder) WhereRaw(sql string, args ...any) *SelectBuilder {
	return b.Where(sql, args...)
}

func (b *SelectBuilder) WhereEq(column string, value any) *SelectBuilder {
	if b.err != nil {
		return b
	}
	col, ok := quoteIdentStrict(b.d, column)
	if !ok {
		b.err = errors.New("corm: invalid column identifier")
		return b
	}
	return b.Where(col+" = ?", value)
}

func (b *SelectBuilder) WhereLike(column string, value any) *SelectBuilder {
	if b.err != nil {
		return b
	}
	col, ok := quoteIdentStrict(b.d, column)
	if !ok {
		b.err = errors.New("corm: invalid column identifier")
		return b
	}
	return b.Where(col+" LIKE ?", value)
}

// WhereIn adds a WHERE IN condition.
// It automatically handles slice arguments.
// Example: WhereIn("id", []int{1, 2, 3})
func (b *SelectBuilder) WhereIn(column string, args ...any) *SelectBuilder {
	return b.WhereExpr(clause.In(column, args...))
}

func (b *SelectBuilder) WhereInIdent(column string, args ...any) *SelectBuilder {
	if b.err != nil {
		return b
	}
	col, ok := quoteIdentStrict(b.d, column)
	if !ok {
		b.err = errors.New("corm: invalid column identifier")
		return b
	}
	return b.WhereExpr(clause.In(col, args...))
}

// WhereExpr adds a clause.Expr as a WHERE condition.
func (b *SelectBuilder) WhereExpr(e clause.Expr) *SelectBuilder {
	if strings.TrimSpace(e.SQL) == "" {
		return b
	}
	b.where = append(b.where, e)
	return b
}

// WhereSubquery adds a condition with a subquery: "column op (subquery)".
func (b *SelectBuilder) WhereSubquery(column, op string, sub *SelectBuilder) *SelectBuilder {
	if b.err != nil {
		return b
	}
	sqlStr, args, err := sub.sqlRaw()
	if err != nil {
		b.err = err
		return b
	}
	return b.Where(column+" "+op+" ("+sqlStr+")", args...)
}

// WhereInSubquery adds a "column IN (subquery)" condition.
func (b *SelectBuilder) WhereInSubquery(column string, sub *SelectBuilder) *SelectBuilder {
	return b.WhereSubquery(column, "IN", sub)
}

// Join adds a JOIN clause.
// Do not concatenate untrusted user input into the joinSQL string.
func (b *SelectBuilder) Join(joinSQL string) *SelectBuilder {
	if strings.TrimSpace(joinSQL) == "" {
		return b
	}
	b.joins = append(b.joins, clause.Raw(joinSQL))
	return b
}

// JoinExpr adds a JOIN clause with a clause.Expr as the ON condition.
func (b *SelectBuilder) JoinExpr(joinType, table string, onExpr clause.Expr) *SelectBuilder {
	if b.err != nil {
		return b
	}
	if strings.TrimSpace(onExpr.SQL) == "" {
		b.err = errors.New("corm: empty join condition")
		return b
	}
	
	// Construct the JOIN clause: "LEFT JOIN table ON condition"
	joinType = strings.TrimSpace(strings.ToUpper(joinType))
	switch joinType {
	case "JOIN", "LEFT JOIN", "RIGHT JOIN", "INNER JOIN", "FULL JOIN", "CROSS JOIN":
	default:
		b.err = errors.New("corm: invalid join type")
		return b
	}
	joinSQL := joinType + " " + quoteMaybe(b.d, table) + " ON " + onExpr.SQL
	b.joins = append(b.joins, clause.Raw(joinSQL, onExpr.Args...))
	return b
}

func (b *SelectBuilder) JoinAs(joinType, table, alias string, onExpr clause.Expr) *SelectBuilder {
	if b.err != nil {
		return b
	}
	if strings.TrimSpace(onExpr.SQL) == "" {
		b.err = errors.New("corm: empty join condition")
		return b
	}
	joinType = strings.TrimSpace(strings.ToUpper(joinType))
	switch joinType {
	case "JOIN", "LEFT JOIN", "RIGHT JOIN", "INNER JOIN", "FULL JOIN":
	default:
		b.err = errors.New("corm: invalid join type")
		return b
	}
	qTable, ok := quoteIdentStrict(b.d, table)
	if !ok {
		b.err = errors.New("corm: invalid table identifier")
		return b
	}
	alias = strings.TrimSpace(alias)
	if !isSimpleIdent(alias) {
		b.err = errors.New("corm: invalid alias identifier")
		return b
	}
	joinSQL := joinType + " " + qTable + " AS " + alias + " ON " + onExpr.SQL
	b.joins = append(b.joins, clause.Raw(joinSQL, onExpr.Args...))
	return b
}

func (b *SelectBuilder) LeftJoinAs(table, alias string, onExpr clause.Expr) *SelectBuilder {
	return b.JoinAs("LEFT JOIN", table, alias, onExpr)
}

func (b *SelectBuilder) RightJoinAs(table, alias string, onExpr clause.Expr) *SelectBuilder {
	return b.JoinAs("RIGHT JOIN", table, alias, onExpr)
}

func (b *SelectBuilder) InnerJoinAs(table, alias string, onExpr clause.Expr) *SelectBuilder {
	return b.JoinAs("INNER JOIN", table, alias, onExpr)
}

func (b *SelectBuilder) FullJoinAs(table, alias string, onExpr clause.Expr) *SelectBuilder {
	return b.JoinAs("FULL JOIN", table, alias, onExpr)
}

// LeftJoinOn adds a LEFT JOIN with a safe expression.
func (b *SelectBuilder) LeftJoinOn(table string, onExpr clause.Expr) *SelectBuilder {
	return b.JoinExpr("LEFT JOIN", table, onExpr)
}

// RightJoinOn adds a RIGHT JOIN with a safe expression.
func (b *SelectBuilder) RightJoinOn(table string, onExpr clause.Expr) *SelectBuilder {
	return b.JoinExpr("RIGHT JOIN", table, onExpr)
}

// InnerJoinOn adds an INNER JOIN with a safe expression.
func (b *SelectBuilder) InnerJoinOn(table string, onExpr clause.Expr) *SelectBuilder {
	return b.JoinExpr("INNER JOIN", table, onExpr)
}

// JoinRaw is an alias for Join.
func (b *SelectBuilder) JoinRaw(joinSQL string) *SelectBuilder {
	return b.Join(joinSQL)
}

// LeftJoin adds a LEFT JOIN clause.
func (b *SelectBuilder) LeftJoin(table, on string) *SelectBuilder {
	return b.Join("LEFT JOIN " + quoteMaybe(b.d, table) + " ON " + on)
}

// RightJoin adds a RIGHT JOIN clause.
func (b *SelectBuilder) RightJoin(table, on string) *SelectBuilder {
	return b.Join("RIGHT JOIN " + quoteMaybe(b.d, table) + " ON " + on)
}

// InnerJoin adds an INNER JOIN clause.
func (b *SelectBuilder) InnerJoin(table, on string) *SelectBuilder {
	return b.Join("INNER JOIN " + quoteMaybe(b.d, table) + " ON " + on)
}

// CrossJoin adds a CROSS JOIN clause.
func (b *SelectBuilder) CrossJoin(table string) *SelectBuilder {
	return b.Join("CROSS JOIN " + quoteMaybe(b.d, table))
}

// FullJoin adds a FULL JOIN clause.
func (b *SelectBuilder) FullJoin(table, on string) *SelectBuilder {
	return b.Join("FULL JOIN " + quoteMaybe(b.d, table) + " ON " + on)
}

// Union adds a UNION clause.
func (b *SelectBuilder) Union(other *SelectBuilder) *SelectBuilder {
	return b.union("UNION", other)
}

// UnionAll adds a UNION ALL clause.
func (b *SelectBuilder) UnionAll(other *SelectBuilder) *SelectBuilder {
	return b.union("UNION ALL", other)
}

func (b *SelectBuilder) union(op string, other *SelectBuilder) *SelectBuilder {
	if b.err != nil {
		return b
	}
	sqlStr, args, err := other.sqlRaw()
	if err != nil {
		b.err = err
		return b
	}
	b.unions = append(b.unions, clause.Raw(op+" "+sqlStr, args...))
	return b
}



// GroupBy adds a GROUP BY clause.
func (b *SelectBuilder) GroupBy(columns ...string) *SelectBuilder {
	b.groupBy = append(b.groupBy, columns...)
	return b
}

// Having adds a HAVING condition.
func (b *SelectBuilder) Having(sql string, args ...any) *SelectBuilder {
	b.having = append(b.having, clause.Raw(sql, args...))
	return b
}

func (b *SelectBuilder) HavingRaw(sql string, args ...any) *SelectBuilder {
	return b.Having(sql, args...)
}

// OrderBy adds an ORDER BY clause.
// dir must be "ASC" or "DESC".
// Do not pass untrusted user input as column/dir.
func (b *SelectBuilder) OrderBy(column, dir string) *SelectBuilder {
	dir = strings.TrimSpace(strings.ToUpper(dir))
	switch dir {
	case "ASC", "DESC":
	default:
		dir = "ASC"
	}
	b.orderBy = append(b.orderBy, quoteMaybe(b.d, column)+" "+dir)
	return b
}

func (b *SelectBuilder) OrderByRaw(raw string) *SelectBuilder {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return b
	}
	b.orderBy = append(b.orderBy, raw)
	return b
}

func (b *SelectBuilder) OrderByIdent(column, dir string) *SelectBuilder {
	if b.err != nil {
		return b
	}
	col, ok := quoteIdentStrict(b.d, column)
	if !ok {
		b.err = errors.New("corm: invalid column identifier")
		return b
	}
	dir = strings.TrimSpace(strings.ToUpper(dir))
	switch dir {
	case "ASC", "DESC":
	default:
		dir = "ASC"
	}
	b.orderBy = append(b.orderBy, col+" "+dir)
	return b
}

// OrderByAsc adds an ORDER BY ASC clause.
func (b *SelectBuilder) OrderByAsc(column string) *SelectBuilder {
	return b.OrderBy(column, "ASC")
}

// OrderByDesc adds an ORDER BY DESC clause.
func (b *SelectBuilder) OrderByDesc(column string) *SelectBuilder {
	return b.OrderBy(column, "DESC")
}

// Limit sets the LIMIT.
func (b *SelectBuilder) Limit(limit int) *SelectBuilder {
	if limit < 0 {
		limit = 0
	}
	b.limit = &limit
	return b
}

// LimitOffset sets LIMIT and OFFSET in SQL order.
func (b *SelectBuilder) LimitOffset(limit, offset int) *SelectBuilder {
	return b.Limit(limit).Offset(offset)
}

// Offset sets the OFFSET.
func (b *SelectBuilder) Offset(offset int) *SelectBuilder {
	if offset < 0 {
		offset = 0
	}
	b.offset = &offset
	return b
}

// SQL generates the SQL query and arguments.
func (b *SelectBuilder) SQL() (string, []any, error) {
	sqlStr, args, err := b.sqlRaw()
	if err != nil {
		return "", nil, err
	}
	rewritten, _ := b.d.RewritePlaceholders(sqlStr, 1)
	return rewritten, args, nil
}

func (b *SelectBuilder) sqlRaw() (string, []any, error) {
	if b.err != nil {
		return "", nil, b.err
	}
	if strings.TrimSpace(b.table) == "" {
		return "", nil, errors.New("corm: missing table for select")
	}

	var buf bytes.Buffer
	buf.Grow(128)

	buf.WriteString("SELECT ")
	if b.distinct {
		buf.WriteString("DISTINCT ")
	}
	if len(b.columns) == 0 {
		buf.WriteString("*")
	} else {
		for i, c := range b.columns {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(quoteMaybe(b.d, c))
		}
	}
	buf.WriteString(" FROM ")
	buf.WriteString(quoteMaybe(b.d, b.table))

	var args []any
	if len(b.tableArgs) > 0 {
		args = append(args, b.tableArgs...)
	}

	for _, j := range b.joins {
		buf.WriteString(" ")
		buf.WriteString(j.SQL)
		args = append(args, j.Args...)
	}

	if len(b.where) > 0 {
		buf.WriteString(" WHERE ")
		wrote := 0
		for _, w := range b.where {
			if strings.TrimSpace(w.SQL) == "" {
				continue
			}
			if wrote > 0 {
				buf.WriteString(" AND ")
			}
			buf.WriteString("(")
			buf.WriteString(w.SQL)
			buf.WriteString(")")
			args = append(args, w.Args...)
			wrote++
		}
		if wrote == 0 {
			buf.Truncate(buf.Len() - len(" WHERE "))
		}
	}

	if len(b.groupBy) > 0 {
		buf.WriteString(" GROUP BY ")
		for i, c := range b.groupBy {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(quoteMaybe(b.d, c))
		}
	}

	if len(b.having) > 0 {
		buf.WriteString(" HAVING ")
		wrote := 0
		for _, h := range b.having {
			if strings.TrimSpace(h.SQL) == "" {
				continue
			}
			if wrote > 0 {
				buf.WriteString(" AND ")
			}
			buf.WriteString("(")
			buf.WriteString(h.SQL)
			buf.WriteString(")")
			args = append(args, h.Args...)
			wrote++
		}
		if wrote == 0 {
			buf.Truncate(buf.Len() - len(" HAVING "))
		}
	}

	if len(b.orderBy) > 0 {
		buf.WriteString(" ORDER BY ")
		for i, o := range b.orderBy {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(o)
		}
	}

	if b.limit != nil {
		buf.WriteString(" LIMIT ?")
		args = append(args, *b.limit)
	}
	if b.offset != nil {
		buf.WriteString(" OFFSET ?")
		args = append(args, *b.offset)
	}

	for _, u := range b.unions {
		if strings.TrimSpace(u.SQL) == "" {
			continue
		}
		buf.WriteString(" ")
		buf.WriteString(u.SQL)
		args = append(args, u.Args...)
	}

	return buf.String(), args, nil
}

func (b *SelectBuilder) Query(ctx context.Context) (*sql.Rows, error) {
	sqlStr, args, err := b.SQL()
	if err != nil {
		return nil, err
	}
	return b.exec.QueryContext(ctx, sqlStr, args...)
}

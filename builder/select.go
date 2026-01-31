package builder

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"

	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/dialect"
)

const (
	selectColumnIdent = iota
	selectColumnExpr
)

type selectColumnItem struct {
	kind  int
	ident string
	expr  clause.Expr
}

// SelectBuilder builds SELECT statements.
type SelectBuilder struct {
	exec      Executor
	d         dialect.Dialect
	columns   []selectColumnItem
	fromTable string
	fromSub   *SelectBuilder
	fromAlias string
	joins     []selectJoinItem
	where     whereBuilder
	groupBy   []string
	having    []clause.Expr
	orderBy   []clause.Expr
	limit     *int
	offset    *int
	distinct  bool
	forUpdate bool
	unions    []selectUnionItem
	err       error
}

// selectBuilderPool reduces allocations by reusing SelectBuilder instances.
var selectBuilderPool = sync.Pool{
	New: func() any {
		return &SelectBuilder{}
	},
}

// maxPooledSelectColumns limits the capacity of columns slice to prevent memory bloat.
const maxPooledSelectColumns = 64

// maxPooledSelectJoins limits the capacity of joins slice to prevent memory bloat.
const maxPooledSelectJoins = 16

func newSelect(exec Executor, d dialect.Dialect, columns []string) *SelectBuilder {
	b := selectBuilderPool.Get().(*SelectBuilder)
	b.exec = exec
	b.d = d
	b.where.d = d
	if cap(b.where.items) < 4 {
		b.where.items = make([]whereItem, 0, 4)
	} else {
		b.where.items = b.where.items[:0]
	}
	// Reset all fields
	if cap(b.columns) < len(columns) {
		b.columns = make([]selectColumnItem, 0, len(columns))
	} else {
		b.columns = b.columns[:0]
	}
	for _, c := range columns {
		b.columns = append(b.columns, selectColumnItem{kind: selectColumnIdent, ident: c})
	}
	b.fromTable = ""
	b.fromSub = nil
	b.fromAlias = ""
	if cap(b.joins) > maxPooledSelectJoins {
		b.joins = nil
	} else {
		b.joins = b.joins[:0]
	}
	if cap(b.groupBy) > 16 {
		b.groupBy = nil
	} else {
		b.groupBy = b.groupBy[:0]
	}
	if cap(b.having) > 8 {
		b.having = nil
	} else {
		b.having = b.having[:0]
	}
	if cap(b.orderBy) > 16 {
		b.orderBy = nil
	} else {
		b.orderBy = b.orderBy[:0]
	}
	b.limit = nil
	b.offset = nil
	b.distinct = false
	b.forUpdate = false
	if cap(b.unions) > 4 {
		b.unions = nil
	} else {
		b.unions = b.unions[:0]
	}
	b.err = nil
	return b
}

// putSelectBuilder returns a SelectBuilder to the pool for reuse.
// This is called internally after SQL() to reduce allocations.
func putSelectBuilder(b *SelectBuilder) {
	if b == nil {
		return
	}
	// Only pool if slices are within reasonable limits
	if cap(b.columns) > maxPooledSelectColumns ||
		cap(b.joins) > maxPooledSelectJoins ||
		cap(b.where.items) > maxPooledWhereItems {
		return
	}
	// Clear references to help GC
	b.exec = nil
	b.d = nil
	b.fromSub = nil
	for i := range b.where.items {
		b.where.items[i].sub = nil
	}
	for i := range b.joins {
		b.joins[i].sub = nil
	}
	for i := range b.unions {
		b.unions[i].sub = nil
	}
	selectBuilderPool.Put(b)
}

const (
	selectJoinExpr = iota
	selectJoinSubquery
)

type selectJoinItem struct {
	kind     int
	expr     clause.Expr
	joinType string
	sub      *SelectBuilder
	alias    string
	on       clause.Expr
}

type selectUnionItem struct {
	op  string
	sub *SelectBuilder
}

// Distinct enables DISTINCT selection.
func (b *SelectBuilder) Distinct() *SelectBuilder {
	b.distinct = true
	return b
}

func (b *SelectBuilder) ForUpdate() *SelectBuilder {
	b.forUpdate = true
	return b
}

// SelectExpr adds columns with expressions (e.g. COUNT(*), AS alias).
func (b *SelectBuilder) SelectExpr(exprs ...clause.Expr) *SelectBuilder {
	if b.err != nil {
		return b
	}
	for _, e := range exprs {
		if strings.TrimSpace(e.SQL) == "" {
			continue
		}
		b.columns = append(b.columns, selectColumnItem{kind: selectColumnExpr, expr: e})
	}
	return b
}

// From sets the table to select from.
func (b *SelectBuilder) From(table string) *SelectBuilder {
	if b.err != nil {
		return b
	}
	q, err := validateTable(b.d, table)
	if err != nil {
		b.err = err
		return b
	}
	b.fromTable = q
	b.fromSub = nil
	b.fromAlias = ""
	return b
}

func (b *SelectBuilder) FromAs(table, alias string) *SelectBuilder {
	if b.err != nil {
		return b
	}
	qTable, err := validateTable(b.d, table)
	if err != nil {
		b.err = err
		return b
	}
	alias = strings.TrimSpace(alias)
	if !isSimpleIdent(alias) {
		b.err = errors.New("corm: invalid alias identifier")
		return b
	}
	b.fromTable = qTable + " AS " + alias
	b.fromSub = nil
	b.fromAlias = ""
	return b
}

// FromSelect sets a subquery as the table to select from.
func (b *SelectBuilder) FromSelect(sub *SelectBuilder, alias string) *SelectBuilder {
	if b.err != nil {
		return b
	}
	if sub == nil {
		b.err = errors.New("corm: nil subquery")
		return b
	}
	alias = strings.TrimSpace(alias)
	if !isSimpleIdent(alias) {
		b.err = errors.New("corm: invalid alias identifier")
		return b
	}
	b.fromTable = ""
	b.fromSub = sub
	b.fromAlias = alias
	return b
}

// Where adds a WHERE condition.
// It supports "id = ?", 1 or "name = ?", "alice".
// Do not concatenate untrusted user input into the SQL string; use args for parameter binding.
func (b *SelectBuilder) Where(sql string, args ...any) *SelectBuilder {
	if b.err != nil {
		return b
	}
	b.where.Where(sql, args...)
	return b
}

func (b *SelectBuilder) WhereEq(column string, value any) *SelectBuilder {
	if b.err != nil {
		return b
	}
	b.where.WhereEq(column, value)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

func (b *SelectBuilder) WhereLike(column string, value any) *SelectBuilder {
	if b.err != nil {
		return b
	}
	b.where.WhereLike(column, value)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

// WhereMap adds conditions in the form of "column = ?" joined by AND.
// Keys are applied in sorted order to keep the generated SQL deterministic.
func (b *SelectBuilder) WhereMap(conditions map[string]any) *SelectBuilder {
	if b.err != nil {
		return b
	}
	b.where.WhereMap(conditions)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

// WhereIn adds a WHERE IN condition.
// It automatically handles slice arguments.
// The column must be a valid identifier; it will be safely quoted.
func (b *SelectBuilder) WhereIn(column string, args ...any) *SelectBuilder {
	if b.err != nil {
		return b
	}
	b.where.WhereIn(column, args...)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

// WhereSubquery adds a condition with a subquery: "column op (subquery)".
func (b *SelectBuilder) WhereSubquery(column, op string, sub *SelectBuilder) *SelectBuilder {
	if b.err != nil {
		return b
	}
	b.where.WhereSubquery(column, op, sub)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

// WhereInSubquery adds a "column IN (subquery)" condition.
func (b *SelectBuilder) WhereInSubquery(column string, sub *SelectBuilder) *SelectBuilder {
	return b.WhereSubquery(column, "IN", sub)
}

// JoinRaw adds a raw JOIN clause.
// Do not concatenate untrusted user input into joinSQL.
func (b *SelectBuilder) JoinRaw(joinSQL string, args ...any) *SelectBuilder {
	if b.err != nil {
		return b
	}
	joinSQL = strings.TrimSpace(joinSQL)
	if joinSQL == "" {
		return b
	}
	b.joins = append(b.joins, selectJoinItem{kind: selectJoinExpr, expr: clause.Expr{SQL: joinSQL, Args: args}})
	return b
}

// WhereExpr adds a structured clause.Expr as a WHERE condition.
// This is useful when you have a pre-built expression.
func (b *SelectBuilder) WhereExpr(e clause.Expr) *SelectBuilder {
	if b.err != nil {
		return b
	}
	b.where.WhereExpr(e)
	if b.where.err != nil {
		b.err = b.where.err
	}
	return b
}

func (b *SelectBuilder) joinOn(joinType, table string, onExpr clause.Expr) *SelectBuilder {
	if b.err != nil {
		return b
	}
	if strings.TrimSpace(onExpr.SQL) == "" {
		b.err = errors.New("corm: empty join condition")
		return b
	}
	qTable, ok := quoteIdentStrict(b.d, table)
	if !ok {
		b.err = errors.New("corm: invalid table identifier")
		return b
	}
	joinSQL := joinType + " " + qTable + " ON " + onExpr.SQL
	b.joins = append(b.joins, selectJoinItem{kind: selectJoinExpr, expr: clause.Expr{SQL: joinSQL, Args: onExpr.Args}})
	return b
}

func (b *SelectBuilder) joinOnAs(joinType, table, alias string, onExpr clause.Expr) *SelectBuilder {
	if b.err != nil {
		return b
	}
	if strings.TrimSpace(onExpr.SQL) == "" {
		b.err = errors.New("corm: empty join condition")
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
	b.joins = append(b.joins, selectJoinItem{kind: selectJoinExpr, expr: clause.Expr{SQL: joinSQL, Args: onExpr.Args}})
	return b
}

func (b *SelectBuilder) joinSelectAs(joinType string, sub *SelectBuilder, alias string, onExpr clause.Expr) *SelectBuilder {
	if b.err != nil {
		return b
	}
	if sub == nil {
		b.err = errors.New("corm: nil subquery")
		return b
	}
	if strings.TrimSpace(onExpr.SQL) == "" {
		b.err = errors.New("corm: empty join condition")
		return b
	}
	alias = strings.TrimSpace(alias)
	if !isSimpleIdent(alias) {
		b.err = errors.New("corm: invalid alias identifier")
		return b
	}
	b.joins = append(b.joins, selectJoinItem{
		kind:     selectJoinSubquery,
		joinType: joinType,
		sub:      sub,
		alias:    alias,
		on:       onExpr,
	})
	return b
}

// Join adds an INNER JOIN clause with arguments.
func (b *SelectBuilder) Join(table string, onExpr clause.Expr) *SelectBuilder {
	return b.joinOn("JOIN", table, onExpr)
}

func (b *SelectBuilder) JoinAs(table, alias string, onExpr clause.Expr) *SelectBuilder {
	return b.joinOnAs("JOIN", table, alias, onExpr)
}

func (b *SelectBuilder) LeftJoin(table string, onExpr clause.Expr) *SelectBuilder {
	return b.joinOn("LEFT JOIN", table, onExpr)
}

func (b *SelectBuilder) RightJoin(table string, onExpr clause.Expr) *SelectBuilder {
	return b.joinOn("RIGHT JOIN", table, onExpr)
}

func (b *SelectBuilder) InnerJoin(table string, onExpr clause.Expr) *SelectBuilder {
	return b.joinOn("INNER JOIN", table, onExpr)
}

func (b *SelectBuilder) FullJoin(table string, onExpr clause.Expr) *SelectBuilder {
	return b.joinOn("FULL JOIN", table, onExpr)
}

func (b *SelectBuilder) LeftJoinAs(table, alias string, onExpr clause.Expr) *SelectBuilder {
	return b.joinOnAs("LEFT JOIN", table, alias, onExpr)
}

func (b *SelectBuilder) RightJoinAs(table, alias string, onExpr clause.Expr) *SelectBuilder {
	return b.joinOnAs("RIGHT JOIN", table, alias, onExpr)
}

func (b *SelectBuilder) InnerJoinAs(table, alias string, onExpr clause.Expr) *SelectBuilder {
	return b.joinOnAs("INNER JOIN", table, alias, onExpr)
}

func (b *SelectBuilder) FullJoinAs(table, alias string, onExpr clause.Expr) *SelectBuilder {
	return b.joinOnAs("FULL JOIN", table, alias, onExpr)
}

// CrossJoin adds a CROSS JOIN clause.
func (b *SelectBuilder) CrossJoin(table string) *SelectBuilder {
	if b.err != nil {
		return b
	}
	qTable, ok := quoteIdentStrict(b.d, table)
	if !ok {
		b.err = errors.New("corm: invalid table identifier")
		return b
	}
	b.joins = append(b.joins, selectJoinItem{kind: selectJoinExpr, expr: clause.Expr{SQL: "CROSS JOIN " + qTable, Args: clause.EmptyArgs()}})
	return b
}

func (b *SelectBuilder) CrossJoinAs(table, alias string) *SelectBuilder {
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
	b.joins = append(b.joins, selectJoinItem{kind: selectJoinExpr, expr: clause.Expr{SQL: "CROSS JOIN " + qTable + " AS " + alias, Args: clause.EmptyArgs()}})
	return b
}

func (b *SelectBuilder) JoinSelectAs(sub *SelectBuilder, alias string, onExpr clause.Expr) *SelectBuilder {
	return b.joinSelectAs("JOIN", sub, alias, onExpr)
}

func (b *SelectBuilder) LeftJoinSelectAs(sub *SelectBuilder, alias string, onExpr clause.Expr) *SelectBuilder {
	return b.joinSelectAs("LEFT JOIN", sub, alias, onExpr)
}

func (b *SelectBuilder) RightJoinSelectAs(sub *SelectBuilder, alias string, onExpr clause.Expr) *SelectBuilder {
	return b.joinSelectAs("RIGHT JOIN", sub, alias, onExpr)
}

func (b *SelectBuilder) InnerJoinSelectAs(sub *SelectBuilder, alias string, onExpr clause.Expr) *SelectBuilder {
	return b.joinSelectAs("INNER JOIN", sub, alias, onExpr)
}

func (b *SelectBuilder) FullJoinSelectAs(sub *SelectBuilder, alias string, onExpr clause.Expr) *SelectBuilder {
	return b.joinSelectAs("FULL JOIN", sub, alias, onExpr)
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
	if other == nil {
		b.err = errors.New("corm: nil subquery")
		return b
	}
	b.unions = append(b.unions, selectUnionItem{op: op, sub: other})
	return b
}

// GroupBy adds a GROUP BY clause.
func (b *SelectBuilder) GroupBy(columns ...string) *SelectBuilder {
	if b.err != nil {
		return b
	}
	if len(b.groupBy) == 0 {
		b.groupBy = make([]string, 0, len(columns))
	}
	for _, c := range columns {
		q, ok := quoteIdentStrict(b.d, c)
		if !ok {
			b.err = errors.New("corm: invalid column identifier")
			return b
		}
		b.groupBy = append(b.groupBy, q)
	}
	return b
}

// Having adds a HAVING condition.
func (b *SelectBuilder) Having(sql string, args ...any) *SelectBuilder {
	if b.err != nil {
		return b
	}
	b.having = append(b.having, clause.Raw(sql, args...))
	return b
}

// OrderBy adds an ORDER BY clause.
// dir must be "ASC" or "DESC".
// Do not pass untrusted user input as column/dir.
func (b *SelectBuilder) OrderBy(column, dir string) *SelectBuilder {
	if b.err != nil {
		return b
	}
	col, ok := quoteIdentStrict(b.d, column)
	if !ok {
		b.err = errors.New("corm: invalid column identifier")
		return b
	}
	dir = strings.TrimSpace(dir)
	// Fast path: avoid ToUpper for common cases
	if dir == "ASC" || dir == "asc" {
		dir = "ASC"
	} else if dir == "DESC" || dir == "desc" {
		dir = "DESC"
	} else {
		dir = strings.ToUpper(dir)
		if dir != "ASC" && dir != "DESC" {
			dir = "ASC"
		}
	}
	b.orderBy = append(b.orderBy, clause.Expr{SQL: col + " " + dir, Args: clause.EmptyArgs()})
	return b
}

// OrderByRaw appends a raw ORDER BY fragment.
// Do not pass untrusted user input; prefer OrderBy/OrderByAsc/OrderByDesc or OrderByExpr.
func (b *SelectBuilder) OrderByRaw(raw string) *SelectBuilder {
	if b.err != nil {
		return b
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return b
	}
	b.orderBy = append(b.orderBy, clause.Expr{SQL: raw})
	return b
}

// OrderByExpr appends an ORDER BY expression that supports args binding.
// Do not concatenate untrusted user input into Expr.SQL.
func (b *SelectBuilder) OrderByExpr(e clause.Expr) *SelectBuilder {
	if b.err != nil {
		return b
	}
	if strings.TrimSpace(e.SQL) == "" {
		return b
	}
	b.orderBy = append(b.orderBy, e)
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
	if b.err != nil {
		return "", nil, b.err
	}
	if b.d == nil {
		return "", nil, errors.New("corm: nil dialect")
	}

	buf := getBuffer()
	defer putBuffer(buf)
	ab := newArgBuilder(b.d, buf)
	defer putArgBuilder(ab)

	if err := b.appendSQL(buf, ab); err != nil {
		return "", nil, err
	}
	return buf.String(), ab.args, nil
}

func (b *SelectBuilder) appendSQL(buf *strings.Builder, ab *argBuilder) error {
	if strings.TrimSpace(b.fromTable) == "" && b.fromSub == nil {
		return errors.New("corm: missing table for select")
	}

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
			switch c.kind {
			case selectColumnIdent:
				q, ok := quoteSelectColumnStrict(b.d, c.ident)
				if !ok {
					return errors.New("corm: invalid select column identifier, use SelectExpr for expressions")
				}
				buf.WriteString(q)
			case selectColumnExpr:
				if err := ab.appendExpr(c.expr); err != nil {
					return err
				}
			default:
				return errors.New("corm: invalid select column kind")
			}
		}
	}
	buf.WriteString(" FROM ")
	if b.fromSub != nil {
		buf.WriteByte('(')
		if err := b.fromSub.appendSQL(buf, ab); err != nil {
			return err
		}
		buf.WriteString(") AS ")
		buf.WriteString(b.fromAlias)
	} else {
		buf.WriteString(b.fromTable)
	}

	for _, j := range b.joins {
		buf.WriteString(" ")
		switch j.kind {
		case selectJoinExpr:
			if err := ab.appendExpr(j.expr); err != nil {
				return err
			}
		case selectJoinSubquery:
			if j.sub == nil {
				return errors.New("corm: nil subquery")
			}
			buf.WriteString(j.joinType)
			buf.WriteByte('(')
			if err := j.sub.appendSQL(buf, ab); err != nil {
				return err
			}
			buf.WriteString(") AS ")
			buf.WriteString(j.alias)
			buf.WriteString(" ON ")
			if err := ab.appendExpr(j.on); err != nil {
				return err
			}
		default:
			return errors.New("corm: invalid join kind")
		}
	}

	if err := b.where.appendWhere(buf, ab); err != nil {
		return err
	}

	if len(b.groupBy) > 0 {
		buf.WriteString(" GROUP BY ")
		for i, c := range b.groupBy {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(c)
		}
	}

	if len(b.having) > 0 {
		buf.WriteString(" HAVING ")
		wrote := 0
		for _, h := range b.having {
			if strings.TrimSpace(h.SQL) == "" {
				return errors.New("corm: empty HAVING expression")
			}
			if wrote > 0 {
				buf.WriteString(" AND ")
			}
			buf.WriteByte('(')
			if err := ab.appendExpr(h); err != nil {
				return err
			}
			buf.WriteByte(')')
			wrote++
		}
	}

	for _, u := range b.unions {
		if u.sub == nil {
			return errors.New("corm: nil subquery")
		}
		buf.WriteByte(' ')
		buf.WriteString(u.op)
		buf.WriteString(" (")
		if err := u.sub.appendSQL(buf, ab); err != nil {
			return err
		}
		buf.WriteByte(')')
	}

	if len(b.unions) > 0 && b.forUpdate {
		return errors.New("corm: FOR UPDATE with UNION is not supported")
	}

	if len(b.orderBy) > 0 {
		buf.WriteString(" ORDER BY ")
		for i := range b.orderBy {
			if i > 0 {
				buf.WriteString(", ")
			}
			if err := ab.appendExpr(b.orderBy[i]); err != nil {
				return err
			}
		}
	}

	if b.limit != nil {
		buf.WriteString(" LIMIT ")
		buf.WriteString(ab.add(*b.limit))
	}
	if b.offset != nil {
		buf.WriteString(" OFFSET ")
		buf.WriteString(ab.add(*b.offset))
	}

	if b.forUpdate {
		buf.WriteString(" FOR UPDATE")
	}

	return nil
}

func (b *SelectBuilder) Query(ctx context.Context) (*sql.Rows, error) {
	if b.exec == nil {
		return nil, errors.New("corm: missing Executor for select")
	}
	sqlStr, args, err := b.SQL()
	if err != nil {
		return nil, err
	}
	return b.exec.QueryContext(ctx, sqlStr, args...)
}

// QueryFunc executes the query and calls fn with the resulting rows.
// It ensures rows are properly closed after fn returns, even if fn panics.
// This is the recommended way to handle query results to prevent resource leaks.
//
// Example:
//
//	err := e.Select("id", "name").From("users").Where("age > ?", 18).QueryFunc(ctx, func(rows *sql.Rows) error {
//		for rows.Next() {
//			var id int
//			var name string
//			if err := rows.Scan(&id, &name); err != nil {
//				return err
//			}
//			// Process id, name
//		}
//		return rows.Err()
//	})
func (b *SelectBuilder) QueryFunc(ctx context.Context, fn func(*sql.Rows) error) error {
	rows, err := b.Query(ctx)
	if err != nil {
		return err
	}
	defer rows.Close()
	return fn(rows)
}

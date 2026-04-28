package builder

import (
	"maps"
	"slices"
	"strings"

	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/dialect"
)

const (
	whereExpr = iota
	whereSubquery
)

type whereItem struct {
	kind   int
	expr   clause.Expr
	column string
	op     string
	sub    *SelectBuilder
}

type whereBuilder struct {
	d     dialect.Dialect
	items []whereItem
	err   error
}

func (wb *whereBuilder) Where(sql string, args ...any) {
	if wb.err != nil {
		return
	}
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return
	}
	if len(wb.items) == 0 {
		wb.items = make([]whereItem, 0, 4)
	}
	wb.items = append(wb.items, whereItem{kind: whereExpr, expr: clause.Raw(sql, args...)})
}

func (wb *whereBuilder) WhereEq(column string, value any) {
	if wb.err != nil {
		return
	}
	col, ok := quoteIdentStrict(wb.d, column)
	if !ok {
		wb.err = errInvalidColumn
		return
	}
	if len(wb.items) == 0 {
		wb.items = make([]whereItem, 0, 4)
	}
	wb.items = append(wb.items, whereItem{kind: whereExpr, expr: clause.Eq(col, value)})
}

func (wb *whereBuilder) WhereIn(column string, args ...any) {
	if wb.err != nil {
		return
	}
	col, ok := quoteIdentStrict(wb.d, column)
	if !ok {
		wb.err = errInvalidColumn
		return
	}
	wb.WhereExpr(clause.In(col, args...))
}

func (wb *whereBuilder) WhereLike(column string, value any) {
	if wb.err != nil {
		return
	}
	col, ok := quoteIdentStrict(wb.d, column)
	if !ok {
		wb.err = errInvalidColumn
		return
	}
	if len(wb.items) == 0 {
		wb.items = make([]whereItem, 0, 4)
	}
	wb.items = append(wb.items, whereItem{kind: whereExpr, expr: clause.Like(col, value)})
}

func (wb *whereBuilder) WhereMap(conditions map[string]any) {
	if wb.err != nil {
		return
	}
	keys := slices.Collect(maps.Keys(conditions))
	slices.Sort(keys)

	for _, k := range keys {
		if _, ok := quoteColumnStrict(wb.d, k); !ok {
			wb.err = errInvalidColumn
			return
		}
		wb.WhereEq(k, conditions[k])
	}
}

func (wb *whereBuilder) WhereSubquery(column, op string, sub *SelectBuilder) {
	if wb.err != nil {
		return
	}
	if sub == nil {
		wb.err = errNilSubquery
		return
	}
	if _, ok := quoteIdentStrict(wb.d, column); !ok {
		wb.err = errInvalidColumn
		return
	}
	op, ok := normalizeSubqueryOp(op)
	if !ok {
		wb.err = errInvalidOperator
		return
	}
	if len(wb.items) == 0 {
		wb.items = make([]whereItem, 0, 4)
	}
	wb.items = append(wb.items, whereItem{kind: whereSubquery, column: column, op: op, sub: sub})
}

func (wb *whereBuilder) WhereInSubquery(column string, sub *SelectBuilder) {
	wb.WhereSubquery(column, "IN", sub)
}

func (wb *whereBuilder) WhereExpr(e clause.Expr) {
	if wb.err != nil {
		return
	}
	if strings.TrimSpace(e.SQL) == "" {
		return
	}
	if len(wb.items) == 0 {
		wb.items = make([]whereItem, 0, 4)
	}
	wb.items = append(wb.items, whereItem{kind: whereExpr, expr: e})
}

func (wb *whereBuilder) WhereNotIn(column string, args ...any) {
	if wb.err != nil {
		return
	}
	col, ok := quoteIdentStrict(wb.d, column)
	if !ok {
		wb.err = errInvalidColumn
		return
	}
	wb.items = append(wb.items, whereItem{kind: whereExpr, expr: clause.NotIn(col, args...)})
}

func (wb *whereBuilder) WhereBetween(column string, lo, hi any) {
	if wb.err != nil {
		return
	}
	col, ok := quoteIdentStrict(wb.d, column)
	if !ok {
		wb.err = errInvalidColumn
		return
	}
	wb.items = append(wb.items, whereItem{kind: whereExpr, expr: clause.Between(col, lo, hi)})
}

func (wb *whereBuilder) WhereNotLike(column string, value any) {
	if wb.err != nil {
		return
	}
	col, ok := quoteIdentStrict(wb.d, column)
	if !ok {
		wb.err = errInvalidColumn
		return
	}
	wb.items = append(wb.items, whereItem{kind: whereExpr, expr: clause.NotLike(col, value)})
}

func (wb *whereBuilder) WhereExists(sub *SelectBuilder) {
	if wb.err != nil {
		return
	}
	if sub == nil {
		wb.err = errNilSubquery
		return
	}
	wb.items = append(wb.items, whereItem{
		kind: whereSubquery,
		op:   "EXISTS",
		sub:  sub,
	})
}

func (wb *whereBuilder) WhereNotExists(sub *SelectBuilder) {
	if wb.err != nil {
		return
	}
	if sub == nil {
		wb.err = errNilSubquery
		return
	}
	wb.items = append(wb.items, whereItem{
		kind: whereSubquery,
		op:   "NOT EXISTS",
		sub:  sub,
	})
}

func (wb *whereBuilder) appendWhere(buf *strings.Builder, ab *argBuilder) error {
	if len(wb.items) == 0 {
		return nil
	}
	buf.WriteString(" WHERE ")
	wrote, err := wb.appendItems(buf, ab)
	if err != nil {
		return err
	}
	if wrote == 0 {
		return errEmptyWhere
	}
	return nil
}

func (wb *whereBuilder) appendAndWhere(buf *strings.Builder, ab *argBuilder) error {
	if len(wb.items) == 0 {
		return nil
	}
	buf.WriteString(" AND ")
	wrote, err := wb.appendItems(buf, ab)
	if err != nil {
		return err
	}
	if wrote == 0 {
		return nil
	}
	return nil
}

func (wb *whereBuilder) appendItems(buf *strings.Builder, ab *argBuilder) (int, error) {
	wrote := 0
	for _, w := range wb.items {
		switch w.kind {
		case whereExpr:
			if strings.TrimSpace(w.expr.SQL) == "" {
				continue
			}
			if wrote > 0 {
				buf.WriteString(" AND ")
			}
			buf.WriteByte('(')
			if err := ab.appendExpr(w.expr); err != nil {
				return 0, err
			}
			buf.WriteByte(')')
			wrote++
		case whereSubquery:
			if w.sub == nil {
				return 0, errNilSubquery
			}
			if wrote > 0 {
				buf.WriteString(" AND ")
			}
			buf.WriteByte('(')
			if err := wb.appendSubqueryItem(buf, ab, w); err != nil {
				return 0, err
			}
			buf.WriteByte(')')
			wrote++
		default:
			return 0, errInvalidWhereKind
		}
	}
	return wrote, nil
}

func (wb *whereBuilder) appendSubqueryItem(buf *strings.Builder, ab *argBuilder, w whereItem) error {
	if w.op == "EXISTS" || w.op == "NOT EXISTS" {
		buf.WriteString(w.op)
		buf.WriteString(" (")
		if err := w.sub.appendSQL(buf, ab); err != nil {
			return err
		}
		buf.WriteByte(')')
		return nil
	}
	col, ok := quoteIdentStrict(wb.d, w.column)
	if !ok {
		return errInvalidColumn
	}
	buf.WriteString(col)
	buf.WriteByte(' ')
	buf.WriteString(w.op)
	buf.WriteString(" (")
	if err := w.sub.appendSQL(buf, ab); err != nil {
		return err
	}
	buf.WriteByte(')')
	return nil
}

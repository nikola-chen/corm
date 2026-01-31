package builder

import (
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/dialect"
)

const (
	whereExpr = iota
	whereSubquery
)

// whereBuilderPool reduces allocations by reusing whereBuilder instances.
var whereBuilderPool = sync.Pool{
	New: func() any {
		return &whereBuilder{
			items: make([]whereItem, 0, 8),
		}
	},
}

// maxPooledWhereItems limits the capacity of items slice to prevent memory bloat.
const maxPooledWhereItems = 64

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

func newWhereBuilder(d dialect.Dialect) *whereBuilder {
	wb := whereBuilderPool.Get().(*whereBuilder)
	wb.d = d
	wb.items = wb.items[:0]
	wb.err = nil
	return wb
}

func putWhereBuilder(wb *whereBuilder) {
	if wb == nil || cap(wb.items) > maxPooledWhereItems {
		return
	}
	wb.d = nil
	wb.err = nil
	// Clear items to help GC
	for i := range wb.items {
		wb.items[i].expr = clause.Expr{}
		wb.items[i].sub = nil
	}
	wb.items = wb.items[:0]
	whereBuilderPool.Put(wb)
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
		wb.err = errors.New("corm: invalid column identifier")
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
		wb.err = errors.New("corm: invalid column identifier")
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
		wb.err = errors.New("corm: invalid column identifier")
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
	keys := make([]string, 0, len(conditions))
	for k := range conditions {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if _, ok := quoteColumnStrict(wb.d, k); !ok {
			wb.err = errors.New("corm: invalid column identifier")
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
		wb.err = errors.New("corm: nil subquery")
		return
	}
	if _, ok := quoteIdentStrict(wb.d, column); !ok {
		wb.err = errors.New("corm: invalid column identifier")
		return
	}
	op, ok := normalizeSubqueryOp(op)
	if !ok {
		wb.err = errors.New("corm: invalid operator")
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

func (wb *whereBuilder) appendWhere(buf *strings.Builder, ab *argBuilder) error {
	if len(wb.items) == 0 {
		return nil
	}
	buf.WriteString(" WHERE ")
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
				return err
			}
			buf.WriteByte(')')
			wrote++
		case whereSubquery:
			if w.sub == nil {
				return errors.New("corm: nil subquery")
			}
			col, ok := quoteIdentStrict(wb.d, w.column)
			if !ok {
				return errors.New("corm: invalid column identifier")
			}
			if wrote > 0 {
				buf.WriteString(" AND ")
			}
			buf.WriteByte('(')
			buf.WriteString(col)
			buf.WriteByte(' ')
			buf.WriteString(w.op)
			buf.WriteString(" (")
			if err := w.sub.appendSQL(buf, ab); err != nil {
				return err
			}
			buf.WriteString("))")
			wrote++
		default:
			return errors.New("corm: invalid where kind")
		}
	}
	if wrote == 0 {
		return errors.New("corm: all WHERE expressions are empty")
	}
	return nil
}

func (wb *whereBuilder) appendAndWhere(buf *strings.Builder, ab *argBuilder) error {
	if len(wb.items) == 0 {
		return nil
	}
	wrote := 0
	for _, w := range wb.items {
		switch w.kind {
		case whereExpr:
			if strings.TrimSpace(w.expr.SQL) == "" {
				continue
			}
			buf.WriteString(" AND (")
			if err := ab.appendExpr(w.expr); err != nil {
				return err
			}
			buf.WriteByte(')')
			wrote++
		case whereSubquery:
			if w.sub == nil {
				return errors.New("corm: nil subquery")
			}
			col, ok := quoteIdentStrict(wb.d, w.column)
			if !ok {
				return errors.New("corm: invalid column identifier")
			}
			buf.WriteString(" AND (")
			buf.WriteString(col)
			buf.WriteByte(' ')
			buf.WriteString(w.op)
			buf.WriteString(" (")
			if err := w.sub.appendSQL(buf, ab); err != nil {
				return err
			}
			buf.WriteString("))")
			wrote++
		default:
			return errors.New("corm: invalid where kind")
		}
	}
	if wrote == 0 {
		return nil
	}
	return nil
}

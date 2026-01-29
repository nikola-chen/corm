package builder

import (
	"bytes"
	"errors"
	"sort"
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
	wb.Where(col+" = ?", value)
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
	wb.Where(col+" LIKE ?", value)
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
	wb.items = append(wb.items, whereItem{kind: whereExpr, expr: e})
}

func (wb *whereBuilder) appendWhere(buf *bytes.Buffer, ab *argBuilder) error {
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
			buf.WriteString("(")
			if err := ab.appendExpr(buf, w.expr); err != nil {
				return err
			}
			buf.WriteString(")")
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
			buf.WriteString("(")
			buf.WriteString(col)
			buf.WriteString(" ")
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
		buf.Truncate(buf.Len() - len(" WHERE "))
	}
	return nil
}

func (wb *whereBuilder) appendAndWhere(buf *bytes.Buffer, ab *argBuilder) error {
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
			if err := ab.appendExpr(buf, w.expr); err != nil {
				return err
			}
			buf.WriteString(")")
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
			buf.WriteString(" ")
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

package builder

import (
	"strings"
	"testing"

	"github.com/nikola-chen/corm/clause"
	"github.com/nikola-chen/corm/dialect"
)

func TestArgBuilderPlaceholderCountMismatch_MySQL(t *testing.T) {
	d := dialect.MustGet("mysql")
	var buf strings.Builder
	ab := newArgBuilder(d, &buf)

	err := ab.appendExpr(clause.Raw("id = ?", 1, 2))
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestArgBuilderPlaceholderCountMismatch_Postgres(t *testing.T) {
	d := dialect.MustGet("postgres")
	var buf strings.Builder
	ab := newArgBuilder(d, &buf)

	err := ab.appendExpr(clause.Raw("id = ? AND name = ?", 1))
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestArgBuilderPostgresStringLiteralQuestionMark(t *testing.T) {
	d := dialect.MustGet("postgres")
	var buf strings.Builder
	ab := newArgBuilder(d, &buf)

	err := ab.appendExpr(clause.Raw("note = '?' AND id = ?", 7))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); got != "note = '?' AND id = $1" {
		t.Fatalf("sql=%q", got)
	}
	if len(ab.args) != 1 || ab.args[0] != 7 {
		t.Fatalf("args=%v", ab.args)
	}
}

func TestArgBuilderPostgresJSONBOperatorConflict(t *testing.T) {
	d := dialect.MustGet("postgres")
	var buf strings.Builder
	ab := newArgBuilder(d, &buf)

	err := ab.appendExpr(clause.Raw("data ? 'k' AND id = ?", 1))
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestArgBuilderMySQLBackslashEscapeInStringLiteral(t *testing.T) {
	d := dialect.MustGet("mysql")
	var buf strings.Builder
	ab := newArgBuilder(d, &buf)

	err := ab.appendExpr(clause.Raw("note = 'don\\'t ?' AND id = ?", 7))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); got != "note = 'don\\'t ?' AND id = ?" {
		t.Fatalf("sql=%q", got)
	}
	if len(ab.args) != 1 || ab.args[0] != 7 {
		t.Fatalf("args=%v", ab.args)
	}
}




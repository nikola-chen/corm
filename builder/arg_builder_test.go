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

func TestTokenizeSQLBlockComment(t *testing.T) {
	d := dialect.MustGet("postgres")
	var buf strings.Builder
	ab := newArgBuilder(d, &buf)

	err := ab.appendExpr(clause.Raw("/* comment ? */ id = ?", 1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); got != "/* comment ? */ id = $1" {
		t.Fatalf("sql=%q", got)
	}
}

func TestTokenizeSQLLineComment(t *testing.T) {
	d := dialect.MustGet("postgres")
	var buf strings.Builder
	ab := newArgBuilder(d, &buf)

	err := ab.appendExpr(clause.Raw("id = ? -- comment ?\nAND name = ?", 1, "test"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); got != "id = $1 -- comment ?\nAND name = $2" {
		t.Fatalf("sql=%q", got)
	}
}

func TestTokenizeSQLDoubleQuotedIdentifier(t *testing.T) {
	d := dialect.MustGet("postgres")
	var buf strings.Builder
	ab := newArgBuilder(d, &buf)

	err := ab.appendExpr(clause.Raw(`"col?" = ?`, 1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); got != `"col?" = $1` {
		t.Fatalf("sql=%q", got)
	}
}

func TestTokenizeSQLDollarQuotedString(t *testing.T) {
	d := dialect.MustGet("postgres")
	var buf strings.Builder
	ab := newArgBuilder(d, &buf)

	err := ab.appendExpr(clause.Raw(`$$hello?$$ AND id = ?`, 1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); got != `$$hello?$$ AND id = $1` {
		t.Fatalf("sql=%q", got)
	}
}

func TestTokenizeSQLTaggedDollarQuote(t *testing.T) {
	d := dialect.MustGet("postgres")
	var buf strings.Builder
	ab := newArgBuilder(d, &buf)

	err := ab.appendExpr(clause.Raw(`$tag$hello?$tag$ AND id = ?`, 1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); got != `$tag$hello?$tag$ AND id = $1` {
		t.Fatalf("sql=%q", got)
	}
}

func TestCountQuestionPlaceholdersNoQuestionMark(t *testing.T) {
	count := countQuestionPlaceholders("SELECT 1", false)
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
}

func TestArgBuilderSQLTooLong(t *testing.T) {
	d := dialect.MustGet("mysql")
	var buf strings.Builder
	ab := newArgBuilder(d, &buf)

	longSQL := strings.Repeat("x", maxSQLLength+1)
	err := ab.appendExpr(clause.Raw(longSQL))
	if err == nil {
		t.Fatalf("expected error for SQL too long")
	}
}

func TestArgBuilderMissingPlaceholders(t *testing.T) {
	d := dialect.MustGet("mysql")
	var buf strings.Builder
	ab := newArgBuilder(d, &buf)

	err := ab.appendExpr(clause.Raw("SELECT 1", 1))
	if err == nil {
		t.Fatalf("expected error for missing placeholders")
	}
}

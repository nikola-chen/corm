package engine

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestEngineOpenWithNilDialect(t *testing.T) {
	db := openEngineTestDB(t)
	_, err := WithDB(db, "unsupported")
	if err == nil {
		t.Fatalf("expected error for unsupported dialect, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported dialect") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEngineWithDBNilDialect(t *testing.T) {
	db := openEngineTestDB(t)
	_, err := WithDB(db, "unsupported")
	if err == nil {
		t.Fatalf("expected error for unsupported dialect, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported dialect") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEngineConfigZeroValues(t *testing.T) {
	db := openEngineTestDB(t)
	e, err := WithDB(db, engineTestDriverName)
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	ctx := context.Background()
	if err := e.Ping(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEngineTransactionPanicRecovery(t *testing.T) {
	db := openEngineTestDB(t)
	e, err := WithDB(db, engineTestDriverName)
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic but got none")
		}
	}()

	_ = e.Transaction(context.Background(), func(tx *Tx) error {
		panic("test panic")
	})
}

func TestEngineTransactionErrorRollback(t *testing.T) {
	db := openEngineTestDB(t)
	e, err := WithDB(db, engineTestDriverName)
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	testErr := errors.New("test error")
	err = e.Transaction(context.Background(), func(tx *Tx) error {
		return testErr
	})
	if !errors.Is(err, testErr) {
		t.Errorf("expected test error, got: %v", err)
	}
}

func TestTxNestedTransactionPanicRecovery(t *testing.T) {
	db := openEngineTestDB(t)
	e, err := WithDB(db, engineTestDriverName)
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	err = e.Transaction(context.Background(), func(tx *Tx) error {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic in nested transaction")
			}
		}()

		return tx.Transaction(context.Background(), func(subTx *Tx) error {
			panic("nested panic")
		})
	})
	if err != nil {
		t.Errorf("unexpected error from outer transaction: %v", err)
	}
}

func TestTxInvalidSavepointName(t *testing.T) {
	invalidNames := []string{
		"",
		"123start",
		"a!",
		strings.Repeat("a", 129),
	}

	for _, name := range invalidNames {
		if isValidSavepointName(name) {
			t.Errorf("expected invalid savepoint name: %s", name)
		}
	}
}

func TestConfigMaxValues(t *testing.T) {
	db := openEngineTestDB(t)
	cfg := Config{
		MaxOpenConns:    1000000,
		MaxIdleConns:    1000000,
		ConnMaxLifetime: 1000000 * time.Hour,
		MaxLogSQLLen:    1000000,
		MaxLogArgsItems: 1000000,
		MaxLogArgsLen:   1000000,
	}

	e, err := WithDB(db, engineTestDriverName, WithConfig(cfg))
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	ctx := context.Background()
	if err := e.Ping(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEngineBuilderMethod(t *testing.T) {
	db := openEngineTestDB(t)
	e, err := WithDB(db, engineTestDriverName)
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	qb := e.Builder()
	if qb == nil {
		t.Fatalf("expected non-nil builder")
	}

	sqlStr, args, err := qb.Select("*").From("users").Where("id = ?", 1).SQL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sqlStr == "" {
		t.Fatalf("expected non-empty SQL")
	}
	if len(args) != 1 || args[0] != 1 {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestEngineMethods(t *testing.T) {
	db := openEngineTestDB(t)
	e, err := WithDB(db, engineTestDriverName)
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	if e.DB() == nil {
		t.Errorf("expected non-nil DB")
	}
	if e.Dialect() == nil {
		t.Errorf("expected non-nil Dialect")
	}

	stats := e.Stats()
	_ = stats

	ctx := context.Background()
	_ = e.Ping(ctx)

	b := e.Select("id", "name")
	if b == nil {
		t.Errorf("expected non-nil SelectBuilder")
	}

	b2 := e.SelectExpr()
	if b2 == nil {
		t.Errorf("expected non-nil SelectExpr builder")
	}

	ib := e.Insert("users")
	if ib == nil {
		t.Errorf("expected non-nil InsertBuilder")
	}

	ub := e.Update("users")
	if ub == nil {
		t.Errorf("expected non-nil UpdateBuilder")
	}

	del := e.Delete("users")
	if del == nil {
		t.Errorf("expected non-nil DeleteBuilder")
	}
}

func TestEngineRawMethods(t *testing.T) {
	db := openEngineTestDB(t)
	e, err := WithDB(db, engineTestDriverName)
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	ctx := context.Background()

	_, err = e.RawExec(ctx, "SELECT 1")
	if err != nil {
		t.Fatalf("RawExec error: %v", err)
	}

	_, err = e.RawQuery(ctx, "SELECT 1")
	if err != nil {
		t.Fatalf("RawQuery error: %v", err)
	}

	err = e.RawQueryFunc(ctx, "SELECT 1", func(rows *sql.Rows) error {
		return nil
	})
	if err != nil {
		t.Fatalf("RawQueryFunc error: %v", err)
	}
}

func TestEngineWithLogger(t *testing.T) {
	db := openEngineTestDB(t)
	logger := &testLogger{}
	e, err := WithDB(db, engineTestDriverName, WithLogger(logger))
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	if e.logger != logger {
		t.Errorf("expected logger to be set")
	}
}

func TestEngineWithConfig(t *testing.T) {
	db := openEngineTestDB(t)
	cfg := Config{LogSQL: true, MaxOpenConns: 10}
	e, err := WithDB(db, engineTestDriverName, WithConfig(cfg))
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	if !e.cfg.LogSQL {
		t.Errorf("expected LogSQL to be true")
	}
	if e.cfg.MaxOpenConns != 10 {
		t.Errorf("expected MaxOpenConns=10, got %d", e.cfg.MaxOpenConns)
	}
}

func TestEngineWithConfigOptionError(t *testing.T) {
	db := openEngineTestDB(t)
	badOpt := func(e *Engine) error {
		return errors.New("bad option")
	}
	_, err := WithDB(db, engineTestDriverName, badOpt)
	if err == nil {
		t.Fatalf("expected error from bad option")
	}
}

func TestTxMethods(t *testing.T) {
	db := openEngineTestDB(t)
	e, err := WithDB(db, engineTestDriverName)
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	ctx := context.Background()
	tx, err := e.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx error: %v", err)
	}

	if tx.Commit() != nil {
		t.Errorf("expected nil error from Commit")
	}

	tx2, err := e.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx error: %v", err)
	}
	if tx2.Rollback() != nil {
		t.Errorf("expected nil error from Rollback")
	}
}

func TestTxBuilderMethods(t *testing.T) {
	db := openEngineTestDB(t)
	e, err := WithDB(db, engineTestDriverName)
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	ctx := context.Background()
	tx, err := e.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx error: %v", err)
	}
	defer tx.Rollback()

	if tx.Select("id") == nil {
		t.Errorf("expected non-nil Select")
	}
	if tx.SelectExpr() == nil {
		t.Errorf("expected non-nil SelectExpr")
	}
	if tx.Insert("users") == nil {
		t.Errorf("expected non-nil Insert")
	}
	if tx.Update("users") == nil {
		t.Errorf("expected non-nil Update")
	}
	if tx.Delete("users") == nil {
		t.Errorf("expected non-nil Delete")
	}
	if tx.Builder() == nil {
		t.Errorf("expected non-nil Builder")
	}
}

func TestTxRawMethods(t *testing.T) {
	db := openEngineTestDB(t)
	e, err := WithDB(db, engineTestDriverName)
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	ctx := context.Background()
	tx, err := e.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx error: %v", err)
	}
	defer tx.Rollback()

	_, err = tx.RawExec(ctx, "CREATE TABLE test (id INTEGER PRIMARY KEY)")
	if err != nil {
		t.Fatalf("RawExec error: %v", err)
	}

	_, err = tx.RawQuery(ctx, "SELECT * FROM test")
	if err != nil {
		t.Fatalf("RawQuery error: %v", err)
	}

	err = tx.RawQueryFunc(ctx, "SELECT * FROM test", func(rows *sql.Rows) error {
		return nil
	})
	if err != nil {
		t.Fatalf("RawQueryFunc error: %v", err)
	}
}

func TestTxTransactionSuccess(t *testing.T) {
	db := openEngineTestDB(t)
	e, err := WithDB(db, engineTestDriverName)
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	ctx := context.Background()
	tx, err := e.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx error: %v", err)
	}
	defer tx.Rollback()

	err = tx.Transaction(ctx, func(nested *Tx) error {
		_, err := nested.RawExec(ctx, "SELECT 1")
		return err
	})
	if err != nil {
		t.Fatalf("nested transaction error: %v", err)
	}
}

func TestTxTransactionDepthLimit(t *testing.T) {
	db := openEngineTestDB(t)
	e, err := WithDB(db, engineTestDriverName)
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	ctx := context.Background()
	tx, err := e.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx error: %v", err)
	}
	defer tx.Rollback()

	var depth int
	var lastErr error
	var recurse func(*Tx) error
	recurse = func(tx *Tx) error {
		depth++
		if depth > maxSavepointDepth+5 {
			return errors.New("should have been stopped by depth limit")
		}
		return tx.Transaction(ctx, func(inner *Tx) error {
			return recurse(inner)
		})
	}
	lastErr = recurse(tx)
	if lastErr == nil {
		t.Fatal("expected depth limit error, got nil")
	}
	if !errors.Is(lastErr, errSavepointDepth) {
		t.Fatalf("expected errSavepointDepth, got: %v", lastErr)
	}
}

func TestTxTransactionRollback(t *testing.T) {
	db := openEngineTestDB(t)
	e, err := WithDB(db, engineTestDriverName)
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	ctx := context.Background()
	tx, err := e.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx error: %v", err)
	}
	defer tx.Rollback()

	testErr := errors.New("rollback test")
	err = tx.Transaction(ctx, func(nested *Tx) error {
		return testErr
	})
	if !errors.Is(err, testErr) {
		t.Fatalf("expected test error, got: %v", err)
	}
}

func TestIsValidSavepointName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"valid", true},
		{"_valid", true},
		{"valid_123", true},
		{"", false},
		{"123start", false},
		{"a!b", false},
		{"a-b", false},
		{"a b", false},
		{strings.Repeat("a", 129), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidSavepointName(tt.name)
			if got != tt.valid {
				t.Errorf("isValidSavepointName(%q) = %v, want %v", tt.name, got, tt.valid)
			}
		})
	}
}

func TestNopLogger(t *testing.T) {
	var l NopLogger
	l.Printf("test %s", "format")
}

func TestStdLogger(t *testing.T) {
	l := StdLogger()
	if l == nil {
		t.Errorf("expected non-nil logger")
	}
}

func TestEngineTransactionCommit(t *testing.T) {
	db := openEngineTestDB(t)
	e, err := WithDB(db, engineTestDriverName)
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	ctx := context.Background()
	err = e.Transaction(ctx, func(tx *Tx) error {
		_, err := tx.RawExec(ctx, "CREATE TABLE test_commit (id INTEGER PRIMARY KEY)")
		return err
	})
	if err != nil {
		t.Fatalf("Transaction error: %v", err)
	}
}

func TestEngineTransactionRollbackOnError(t *testing.T) {
	db := openEngineTestDB(t)
	e, err := WithDB(db, engineTestDriverName)
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	ctx := context.Background()
	testErr := errors.New("intentional error")
	err = e.Transaction(ctx, func(tx *Tx) error {
		_, err := tx.RawExec(ctx, "CREATE TABLE test_rollback (id INTEGER PRIMARY KEY)")
		if err != nil {
			return err
		}
		return testErr
	})
	if !errors.Is(err, testErr) {
		t.Fatalf("expected test error, got: %v", err)
	}
}

func TestWrapExecutor(t *testing.T) {
	db := openEngineTestDB(t)

	exec := wrapExecutor(db, nil, Config{})
	if exec != db {
		t.Errorf("expected inner executor when logger is nil")
	}

	logger := &testLogger{}
	exec = wrapExecutor(db, logger, Config{})
	if exec != db {
		t.Errorf("expected inner executor when logging is disabled")
	}

	exec = wrapExecutor(db, logger, Config{LogSQL: true})
	if _, ok := exec.(*loggingExecutor); !ok {
		t.Errorf("expected *loggingExecutor when LogSQL is true")
	}

	exec = wrapExecutor(db, logger, Config{SlowQuery: time.Millisecond})
	if _, ok := exec.(*loggingExecutor); !ok {
		t.Errorf("expected *loggingExecutor when SlowQuery is set")
	}
}

func TestTxExecutor(t *testing.T) {
	db := openEngineTestDB(t)
	logger := &testLogger{}
	e, err := WithDB(db, engineTestDriverName, WithLogger(logger))
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	ctx := context.Background()
	tx, err := e.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx error: %v", err)
	}
	defer tx.Rollback()

	exec := tx.executor()
	if exec == nil {
		t.Errorf("expected non-nil executor")
	}
}

func TestEngineExecutor(t *testing.T) {
	db := openEngineTestDB(t)
	logger := &testLogger{}
	e, err := WithDB(db, engineTestDriverName, WithLogger(logger))
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	exec := e.executor()
	if exec == nil {
		t.Errorf("expected non-nil executor")
	}
}

func TestLoggingExecutor(t *testing.T) {
	db := openEngineTestDB(t)
	logger := &testLogger{}
	e, err := WithDB(db, engineTestDriverName, WithLogger(logger), WithConfig(Config{LogSQL: true, LogArgs: true}))
	if err != nil {
		t.Fatalf("WithDB error: %v", err)
	}
	defer e.Close()

	ctx := context.Background()
	_, err = e.RawExec(ctx, "SELECT 1", "arg1", 42)
	if err != nil {
		t.Fatalf("RawExec error: %v", err)
	}

	if len(logger.logs) == 0 {
		t.Errorf("expected logs to be recorded")
	}
}

type testLogger struct {
	logs []string
}

func (l *testLogger) Printf(format string, args ...any) {
	l.logs = append(l.logs, format)
}

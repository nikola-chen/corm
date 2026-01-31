package engine

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

)

func TestEngineOpenWithNilDialect(t *testing.T) {
	// Test opening with unsupported dialect using WithDB
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Skip("sqlite3 not available")
	}
	defer db.Close()

	_, err = WithDB(db, "unsupported")
	if err == nil {
		t.Fatalf("expected error for unsupported dialect, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported dialect") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEngineWithDBNilDialect(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Skip("sqlite3 not available")
	}
	defer db.Close()

	_, err = WithDB(db, "unsupported")
	if err == nil {
		t.Fatalf("expected error for unsupported dialect, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported dialect") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEngineConfigZeroValues(t *testing.T) {
	// Test that zero config values don't cause issues
	e, err := Open("mysql", "user:pass@tcp(localhost:3306)/test?parseTime=true")
	if err != nil {
		t.Skip("mysql not available, skipping test")
	}
	defer e.Close()

	// Should not panic or error
	ctx := context.Background()
	err = e.Ping(ctx)
	if err != nil && !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEngineTransactionPanicRecovery(t *testing.T) {
	e, err := Open("mysql", "user:pass@tcp(localhost:3306)/test?parseTime=true")
	if err != nil {
		t.Skip("mysql not available, skipping test")
	}
	defer e.Close()

	// Test that panic in transaction is recovered and rolled back
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic but got none")
		}
	}()

	err = e.Transaction(context.Background(), func(tx *Tx) error {
		panic("test panic")
	})
	if err == nil {
		t.Errorf("expected error after panic recovery")
	}
}

func TestEngineTransactionErrorRollback(t *testing.T) {
	e, err := Open("mysql", "user:pass@tcp(localhost:3306)/test?parseTime=true")
	if err != nil {
		t.Skip("mysql not available, skipping test")
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
	e, err := Open("mysql", "user:pass@tcp(localhost:3306)/test?parseTime=true")
	if err != nil {
		t.Skip("mysql not available, skipping test")
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
	e, err := Open("mysql", "user:pass@tcp(localhost:3306)/test?parseTime=true")
	if err != nil {
		t.Skip("mysql not available, skipping test")
	}
	defer e.Close()

	err = e.Transaction(context.Background(), func(tx *Tx) error {
		// Test with invalid savepoint names
		invalidNames := []string{
			"",           // empty
			"123start",   // starts with digit
			"a!",         // contains special char
			strings.Repeat("a", 129), // too long
		}

		for _, name := range invalidNames {
			// Manually set savepointSeq to test validation
			tx.savepointSeq++
			savepointName := name
			if savepointName == "" {
				savepointName = "sp_" + strconv.Itoa(tx.savepointSeq)
			}

			if isValidSavepointName(savepointName) {
				t.Errorf("expected invalid savepoint name: %s", savepointName)
			}
		}

		return nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfigMaxValues(t *testing.T) {
	// Test that max values are handled correctly
	cfg := Config{
		MaxOpenConns:    1000000,
		MaxIdleConns:    1000000,
		ConnMaxLifetime: 1000000 * time.Hour,
		MaxLogSQLLen:    1000000,
		MaxLogArgsItems: 1000000,
		MaxLogArgsLen:   1000000,
	}

	e, err := Open("mysql", "user:pass@tcp(localhost:3306)/test?parseTime=true", WithConfig(cfg))
	if err != nil {
		t.Skip("mysql not available, skipping test")
	}
	defer e.Close()

	// Should not panic
	ctx := context.Background()
	err = e.Ping(ctx)
	if err != nil && !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEngineBuilderMethod(t *testing.T) {
	e, err := Open("mysql", "user:pass@tcp(localhost:3306)/test?parseTime=true")
	if err != nil {
		t.Skip("mysql not available, skipping test")
	}
	defer e.Close()

	qb := e.Builder()
	if qb == nil {
		t.Fatalf("expected non-nil builder")
	}

	// Test that builder works
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

func TestTxBuilderMethod(t *testing.T) {
	e, err := Open("mysql", "user:pass@tcp(localhost:3306)/test?parseTime=true")
	if err != nil {
		t.Skip("mysql not available, skipping test")
	}
	defer e.Close()

	err = e.Transaction(context.Background(), func(tx *Tx) error {
		qb := tx.Builder()
		if qb == nil {
			return errors.New("expected non-nil builder")
		}

		sqlStr, args, err := qb.Select("*").From("users").Where("id = ?", 1).SQL()
		if err != nil {
			return err
		}
		if sqlStr == "" {
			return errors.New("expected non-empty SQL")
		}
		if len(args) != 1 || args[0] != 1 {
			return errors.New("unexpected args")
		}
		return nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEngineStats(t *testing.T) {
	e, err := Open("mysql", "user:pass@tcp(localhost:3306)/test?parseTime=true")
	if err != nil {
		t.Skip("mysql not available, skipping test")
	}
	defer e.Close()

	stats := e.Stats()
	if stats.MaxOpenConnections == 0 {
		t.Errorf("expected MaxOpenConnections to be set, got 0")
	}
	
	// Test that stats are updated after operations
	ctx := context.Background()
	err = e.Ping(ctx)
	if err != nil && !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Stats should still be valid
	stats2 := e.Stats()
	if stats2.MaxOpenConnections == 0 {
		t.Errorf("expected MaxOpenConnections to be set after ping, got 0")
	}
}

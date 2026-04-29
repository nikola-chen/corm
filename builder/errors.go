package builder

import (
	"errors"
	"fmt"
)

var (
	errInvalidColumn     = errors.New("corm: invalid column identifier")
	errInvalidTable      = errors.New("corm: invalid table identifier")
	errNilDialect        = errors.New("corm: nil dialect")
	errNilSubquery       = errors.New("corm: nil subquery")
	errMissingTable      = errors.New("corm: missing table name")
	errMissingExecutor   = errors.New("corm: missing Executor")
	errInvalidOperator   = errors.New("corm: invalid operator")
	errInvalidAlias      = errors.New("corm: invalid alias identifier")
	errEmptyJoinCond     = errors.New("corm: empty join condition")
	errEmptyHaving       = errors.New("corm: empty HAVING expression")
	errEmptyWhere        = errors.New("corm: all WHERE expressions are empty")
	errInvalidWhereKind  = errors.New("corm: invalid where kind")
	errInvalidJoinKind   = errors.New("corm: invalid join kind")
	errSelectColumnIdent = errors.New("corm: invalid select column identifier, use SelectExpr for expressions")
	errSelectColumnKind  = errors.New("corm: invalid select column kind")
	errForUpdateUnion    = errors.New("corm: FOR UPDATE with UNION is not supported")
	errTableNameTooLong  = errors.New("corm: table name exceeds maximum length of 128 characters")
	errBatchAfterSet     = errors.New("corm: cannot switch to batch update after using Set/Where")
	errBatchSet          = errors.New("corm: cannot use Set on batch update, use Models/Maps")
	errBatchSetExpr      = errors.New("corm: cannot use SetExpr on batch update, use Models/Maps")
	errBatchIncr         = errors.New("corm: cannot use Increment on batch update, use Models/Maps")
	errBatchDecr         = errors.New("corm: cannot use Decrement on batch update, use Models/Maps")
	errBatchMap          = errors.New("corm: cannot use Map(map) on batch update, use Maps([]map)")
	errBatchLimit        = errors.New("corm: cannot use Limit on batch update")

	errConflictColumn      = errors.New("corm: invalid column identifier in conflict clause")
	errConflictDoNothing   = errors.New("corm: DoNothing() is only supported on Postgres; use InsertIgnore() or raw suffix on MySQL")
	errConflictDialect     = errors.New("corm: unsupported dialect for OnConflict")
	errConflictNoCols      = errors.New("corm: Postgres requires constraint columns for ON CONFLICT DO UPDATE")
	errInsertIgnoreDialect = errors.New("corm: InsertIgnore only supported by MySQL; use OnConflict().DoNothing() for PostgreSQL")

	errMissingSetValues   = errors.New("corm: missing set values for update")
	errUpdateNoWhere      = errors.New("corm: update without WHERE is not allowed, use AllowEmptyWhere() to override")
	errDeleteNoWhere      = errors.New("corm: DELETE without WHERE is not allowed, use AllowEmptyWhere() to override")
	errInvalidLimit       = errors.New("corm: invalid LIMIT value")
	errUpdateLimitDialect = errors.New("corm: UPDATE LIMIT is only supported by MySQL")
	errDeleteLimitDialect = errors.New("corm: DELETE LIMIT is only supported by MySQL")

	errBatchKeyColumn    = errors.New("corm: invalid batch update key column")
	errBatchKeyIncluded  = errors.New("corm: batch update cannot include key column")
	errBatchMissingRows  = errors.New("corm: missing batch rows for update")
	errBatchRowsMismatch = errors.New("corm: batch update rows mismatch")
	errBatchMissingCols  = errors.New("corm: missing columns for batch update")
	errBatchNilMap       = errors.New("corm: nil map in batch update")
	errBatchModelType    = errors.New("corm: batch update models must be of the same type")

	errMissingInsertCols   = errors.New("corm: missing columns for insert")
	errMissingInsertValues = errors.New("corm: missing values for insert")
	errInsertValueMismatch = errors.New("corm: insert values length mismatch columns")
	errMissingInsertExec   = errors.New("corm: missing Executor for insert")
	errInsertReturnSingle  = errors.New("corm: insert returning id requires single-row insert")

	errMissingPlaceholders = errors.New("corm: missing placeholders for args")
	errNilAPI              = errors.New("corm: nil api")
	errEmptyDialect        = errors.New("corm: empty dialect")

	errNegUint      = errors.New("corm: negative value cannot be assigned to uint")
	errNegUint64    = errors.New("corm: negative value cannot be assigned to uint64")
	errNilPtr       = errors.New("corm: dest must be non-nil pointer")
	errIntOverflow  = errors.New("corm: dest int overflow")
	errUintOverflow = errors.New("corm: dest uint overflow")
	errIntPtr       = errors.New("corm: dest must be integer pointer")

	errPGJsonbArrayOp       = errors.New("corm: postgres jsonb operator '?|/?&' conflicts with placeholder '?', use jsonb_exists_any/jsonb_exists_all")
	errPGJsonbOp            = errors.New("corm: postgres jsonb operator '?' conflicts with placeholder '?', use jsonb_exists")
	errInsertOneNoReturning = errors.New("corm: insert.One requires Returning(...)")
	errInsertOneDialect     = errors.New("corm: dialect does not support returning")
	errSQLTooLong           = errors.New("corm: SQL statement exceeds maximum length of 1MB")
)

func unknownColumnErr(col string) error { return fmt.Errorf("corm: unknown column in Model: %s", col) }
func missingColumnValueErr(col string) error {
	return fmt.Errorf("corm: missing value for column: %s", col)
}
func returningDialectErr(dialect string) error {
	return fmt.Errorf("corm: RETURNING is not supported by %s", dialect)
}
func batchModelKindErr(kind string) error {
	return fmt.Errorf("corm: batch update models must be a slice of structs, got %s", kind)
}
func batchMissingKeyInMapErr(col string) error {
	return fmt.Errorf("corm: missing key column in map: %s", col)
}
func batchUnknownColumnErr(col string) error {
	return fmt.Errorf("corm: unknown column in batch update: %s", col)
}
func batchReadonlyColumnErr(col string) error {
	return fmt.Errorf("corm: readonly column in batch update: %s", col)
}
func batchAutoColumnErr(col string) error {
	return fmt.Errorf("corm: auto column in batch update: %s", col)
}
func batchPrimaryKeyColumnErr(col string) error {
	return fmt.Errorf("corm: primary key column in batch update: %s", col)
}
func unsupportedDialectErr(driver string) error {
	return fmt.Errorf("corm: unsupported dialect: %s", driver)
}

I will optimize the `corm` library to improve performance, correctness, and usability, addressing the issues found during analysis.

## Optimization Plan

### 1. Correctness & Performance: Direct Struct Scanning
**Location**: `exec/scan.go`
**Issue**: Currently, `Scan` reads into `interface{}` buffers and then manually converts/assigns to struct fields via reflection. This bypasses the optimized type conversion logic in `database/sql` drivers (e.g., converting `[]byte` from MySQL to `int`), leading to potential bugs and performance overhead.
**Fix**: Rewrite `scanStructRow` to create a slice of pointers directly to the struct fields (`dest ...any` arguments for `rows.Scan`).
- Match columns from `rows.Columns()` with `schema.Fields`.
- Pass `field.Addr().Interface()` to `rows.Scan`.
- Use a dummy target for unknown columns to avoid scan errors.

### 2. Usability: Struct Support in Insert/Update
**Location**: `builder/insert.go`, `builder/update.go`, `schema/schema.go`
**Issue**: `Insert` and `Update` builders currently only support manual column/value pairs (`Values`, `Set`). Users typically expect to pass a struct.
**Fix**:
- **Schema Layer**: Add `Extract(v any) map[string]any` or similar helper in `schema` package to extract db-tag-mapped values from a struct.
- **InsertBuilder**: Add `Record(dest any)` method.
    - Parses the struct schema.
    - Automatically sets `Columns` and adds `Values` from the struct fields.
- **UpdateBuilder**: Add `SetModel(dest any)` method.
    - Parses the struct schema.
    - Automatically adds `Set` entries for all non-ignored fields.

### 3. Robustness: SQL Generation Improvements
**Location**: `builder/internal.go`, `builder/select.go`
**Issue**:
- `OrderBy` clauses don't apply quoting to column names.
- `RewritePlaceholders` in Postgres dialect is basic (acceptable for now but can be slightly cleaner).
**Fix**:
- Apply `quoteMaybe` to column names in `OrderBy`.

## Implementation Steps

1.  **Refactor `exec/scan.go`**: Implement direct pointer scanning for `ScanAll` and `ScanOne`.
2.  **Enhance `schema` package**: Add value extraction logic (`RecordValues` helper).
3.  **Update `builder` package**:
    - Import `corm/schema`.
    - Implement `InsertBuilder.Record(dest any)`.
    - Implement `UpdateBuilder.SetModel(dest any)`.
    - Fix `OrderBy` quoting.
4.  **Verification**: Add a test case that inserts a struct, updates it using a struct, and selects it back, verifying type conversions (e.g., handling string-to-int if driver returns bytes).
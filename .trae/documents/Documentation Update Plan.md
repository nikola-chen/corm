# Documentation Update Plan

I will update both `README.md` and `README_CN.md` to include a comprehensive "All-in-One Example" section. This will showcase the full capabilities of the library as requested.

## 1. README.md (English)
- Add **"Comprehensive Example"** section.
- Include a complete code block demonstrating:
  - **Configuration**: `LogSQL`, `MaxOpenConns`.
  - **Schema Definition**: All tags (`pk`, `readonly`, `omitempty`, `auto`).
  - **Advanced Select**: `Join`, `WhereIn`, `GroupBy`, `Having`, `OrderBy`, `Limit`, `Offset`.
  - **Insert with Return**: Using `Returning` and `One`.
  - **Batch Insert**: Using `Values` multiple times.
  - **Update**: Using `SetMap` and `SetStruct`.
  - **Transaction**: Full transaction lifecycle.

## 2. README_CN.md (Chinese)
- Add **"综合使用示例"** section.
- Provide the same comprehensive code example with Chinese comments.
- Ensure all API methods are covered and explained in context.

## 3. Implementation Details
- I will verify that the example code is syntactically correct and matches the actual API signature (e.g., `InsertBuilder.One` exists).
- I will format the code clearly.

## 4. Execution
- Edit `README.md`.
- Edit `README_CN.md`.

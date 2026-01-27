I will create a lightweight, high-performance ORM library named `corm` (Simple Go ORM) that supports MySQL and PostgreSQL with a chainable API.

## Project Structure
```text
corm/
├── go.mod
├── corm.go           # Engine entry point and connection management
├── session/          # Chainable API implementation
│   ├── session.go    # Core session logic
│   ├── transaction.go# Transaction support
│   └── hooks.go      # Optional: Hooks for Before/After events
├── clause/           # SQL generation engine
│   ├── clause.go     # Clause definitions
│   └── generator.go  # SQL generators (SELECT, INSERT, etc.)
├── dialect/          # Database driver abstraction
│   ├── dialect.go    # Dialect interface
│   ├── mysql.go      # MySQL implementation
│   └── postgres.go   # PostgreSQL implementation
├── schema/           # Struct reflection and caching
│   └── schema.go
└── log/              # Simple logger
```

## Implementation Steps

1.  **Project Initialization**: Initialize `go.mod`.
2.  **Log & Dialect Layer**: Implement a simple logger and the `Dialect` interface to handle SQL differences (e.g., parameter placeholders `?` vs `$1`).
3.  **Schema Layer**: Implement struct parsing using reflection to map Go structs to database tables/columns, including caching for performance.
4.  **Clause Layer**: Implement a flexible SQL builder that can compose clauses like `SELECT`, `WHERE`, `ORDER BY` into a final SQL query.
5.  **Session Layer**: Implement the chainable API (`Select`, `From`, `Where`, etc.) that the user requested. This will use the Clause layer to build queries and the Dialect layer to execute them.
    - API: `Select()`, `From()`, `Where()`, `OrderBy()`, `Limit()`, `Insert()`, `Update()`, `Delete()`.
6.  **Engine Layer**: Implement the main entry point to manage `database/sql` connections and create Sessions.
7.  **Verification**: Create a test file `corm_test.go` with a real SQLite/Mock example (or instructions for MySQL/PG) to verify the chainable API and CRUD operations.

## Key Features
- **Chainable API**: Fluent interface as requested.
- **Cross-DB Support**: Abstraction for MySQL and PostgreSQL.
- **Performance**: Schema caching to minimize reflection overhead.
- **Safety**: Parameterized queries to prevent SQL injection.

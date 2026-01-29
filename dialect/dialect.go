package dialect

import "sync"

// Dialect defines the interface for database dialects.
type Dialect interface {
	// Name returns the name of the dialect.
	Name() string
	// Placeholder returns the placeholder string for the n-th argument.
	Placeholder(n int) string
	// QuoteIdent quotes an identifier.
	QuoteIdent(ident string) string
	// SupportsReturning reports whether the dialect supports the RETURNING clause.
	SupportsReturning() bool
}

var (
	mu       sync.RWMutex
	dialects = map[string]Dialect{}
)

// Register registers a dialect for a driver.
func Register(driverName string, d Dialect) {
	mu.Lock()
	defer mu.Unlock()
	dialects[driverName] = d
}

// Get returns the dialect for a driver.
func Get(driverName string) (Dialect, bool) {
	mu.RLock()
	defer mu.RUnlock()
	d, ok := dialects[driverName]
	return d, ok
}

// MustGet returns the dialect for a driver or panics if it is not registered.
func MustGet(driverName string) Dialect {
	d, ok := Get(driverName)
	if !ok || d == nil {
		panic("corm: unsupported dialect: " + driverName)
	}
	return d
}

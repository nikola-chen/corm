package engine

import "errors"

var (
	errEngineNotInit   = errors.New("corm: engine is not initialized")
	errContextCanceled  = errors.New("corm: context canceled")
	errSavepointDepth   = errors.New("corm: nested transaction depth exceeds limit (32)")
)

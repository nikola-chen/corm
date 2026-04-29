package scan

import (
	"errors"
	"fmt"
)

var (
	errNilPtr             = errors.New("corm: dest must be non-nil pointer")
	errSlicePtr           = errors.New("corm: dest must be pointer to slice")
	errMapStringKeys      = errors.New("corm: map element must have string keys")
	errSliceElemKind      = errors.New("corm: slice element must be struct, *struct, or map")
	errMapStringKey       = errors.New("corm: dest map key must be string")
	errStructMap          = errors.New("corm: dest must be struct/*struct or map/*map")
	errNilInterfaceDest   = errors.New("corm: dest type cannot be nil interface")
	errStructOrMapDest    = errors.New("corm: dest must be struct, *struct, or map")
)

func duplicateColumnErr(col string) error {
	return fmt.Errorf("corm: duplicate column name after normalization: %s, use AS to alias", col)
}

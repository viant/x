package shape

import (
	"fmt"
	"go/types"
)

// IsComparable reports whether all values of the supplied type are safe map
// keys. It resolves named underlying types through the same descriptor universe
// as generic constraints. Interfaces (including fields containing interfaces)
// are excluded because their dynamic values need not be comparable.
func (r Resolver) IsComparable(source string) (result bool, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			result = false
			err = fmt.Errorf("comparable type authority: %v", failure)
		}
	}()
	value, err := newGoTypeAuthority().expression(r, source, nil)
	if err != nil {
		return false, err
	}
	constraint := types.Universe.Lookup("comparable").Type().Underlying().(*types.Interface)
	return types.Implements(value, constraint), nil
}

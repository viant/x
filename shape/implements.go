package shape

import (
	"fmt"
	"reflect"
)

// Implements checks the requested value/pointer method set against a runtime
// interface contract. Linked receivers use Go's exact implementation test;
// synthetic receivers use the canonical exported method-set owner.
func (t *Type) Implements(contract reflect.Type, pointer bool) (bool, error) {
	if t == nil || t.descriptor == nil {
		return false, fmt.Errorf("receiver type authority is required")
	}
	if contract == nil || contract.Kind() != reflect.Interface {
		return false, fmt.Errorf("implementation contract must be an interface")
	}
	if receiver := t.descriptor.Type; receiver != nil {
		// Preserve defined pointer types; only unnamed wrappers select their
		// named value receiver. Such defined pointers do not inherit methods.
		for receiver.Kind() == reflect.Pointer && receiver.Name() == "" {
			receiver = receiver.Elem()
		}
		if pointer {
			receiver = reflect.PointerTo(receiver)
		}
		return receiver.Implements(contract), nil
	}
	for index := 0; index < contract.NumMethod(); index++ {
		if !contract.Method(index).IsExported() {
			return false, fmt.Errorf("unexported interface methods require linked receiver authority")
		}
	}
	actual, err := t.Methods(pointer)
	if err != nil {
		return false, err
	}
	expected, err := Linked(contract).Methods(false)
	if err != nil {
		return false, err
	}
	for _, required := range expected {
		found := false
		for _, method := range actual {
			if method.Name != required.Name {
				continue
			}
			if method.Variadic != required.Variadic || len(method.Parameters) != len(required.Parameters) || len(method.Results) != len(required.Results) {
				return false, nil
			}
			for _, pair := range [][2][]string{{method.Parameters, required.Parameters}, {method.Results, required.Results}} {
				for index := range pair[0] {
					equal, err := (Contract{}).Equivalent(pair[0][index], pair[1][index])
					if err != nil {
						return false, err
					}
					if !equal {
						return false, nil
					}
				}
			}
			found = true
			break
		}
		if !found {
			return false, nil
		}
	}
	return true, nil
}

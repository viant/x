package shape

import (
	"fmt"
	"reflect"
)

// GetOptional reads a compiled field without allocating nil ancestors. Present
// is false only for an absent root/ancestor; a nil leaf is a present typed value.
// Wrong owner types and inaccessible/invalid paths remain errors. Collection
// element paths requiring indexes must use GetAt instead.
func (a *Accessor) GetOptional(target any) (value reflect.Value, present bool, err error) {
	if a == nil || a.owner == nil || a.field.Type == nil {
		return reflect.Value{}, false, fmt.Errorf("compiled field accessor is required")
	}
	for _, step := range a.steps {
		if step.collections != 0 {
			return reflect.Value{}, false, fmt.Errorf("optional field access requires collection indexes; use GetAt")
		}
	}
	if target == nil {
		return reflect.Value{}, false, nil
	}
	if owner := (Runtime{}).Indirect(reflect.TypeOf(target)); owner != a.owner {
		return reflect.Value{}, false, fmt.Errorf("field target must be %s", a.owner)
	}
	// GetAt already implements checked, nonallocating traversal and represents
	// absent ancestors with an invalid Value. Adapt a simple compiled path into
	// one indexed step without changing the accessor or copying its metadata.
	reader := a
	if len(a.steps) == 0 {
		copy := *a
		copy.steps = []indexedField{{index: a.field.Index}}
		reader = &copy
	}
	value, err = reader.GetAt(target)
	if err != nil {
		return reflect.Value{}, false, err
	}
	return value, value.IsValid(), nil
}

// Equal compares optional field values. Two absent paths are equal; an absent
// path differs from a present nil/zero leaf. Neither input is modified.
func (a *Accessor) Equal(left, right any) (bool, error) {
	first, firstPresent, err := a.GetOptional(left)
	if err != nil {
		return false, err
	}
	second, secondPresent, err := a.GetOptional(right)
	if err != nil {
		return false, err
	}
	if firstPresent != secondPresent {
		return false, nil
	}
	if !firstPresent {
		return true, nil
	}
	return reflect.DeepEqual(first.Interface(), second.Interface()), nil
}

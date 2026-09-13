package shape

import (
	"fmt"
	"reflect"
)

// Collection normalizes exact typed values without mapping or converting their
// fields. It is independent of collection identity/association policy.
type Collection[T any] struct{}

// Pointers returns rows in source order. It accepts T, pointers to T, and
// slices/arrays (including named containers) of T or pointers to T. Pointers to
// collection containers are also accepted. Distinct entity types are rejected.
// Nil containers return nil; nonnil empty containers return an empty slice.
// Nil pointer elements retain their ordinal. Slice and pointer-array elements
// retain their addresses. Scalar values and value arrays require shallow copies
// for addressability; this operation is not a detached snapshot or identity map.
func (Collection[T]) Pointers(value any) ([]*T, error) {
	if value == nil {
		return nil, nil
	}
	entity := reflect.TypeOf((*T)(nil)).Elem()
	pointer := reflect.PointerTo(entity)
	v := reflect.ValueOf(value)
	if v.Type() == entity {
		copy := value.(T)
		return []*T{&copy}, nil
	}
	if v.Kind() == reflect.Pointer && v.Type().Elem() == entity && v.Type().ConvertibleTo(pointer) {
		if v.IsNil() {
			return nil, nil
		}
		return []*T{v.Convert(pointer).Interface().(*T)}, nil
	}
	container := v.Type()
	seen := make(map[reflect.Type]bool)
	for container.Kind() == reflect.Pointer {
		if seen[container] {
			return nil, fmt.Errorf("collection %T has a recursive pointer type", value)
		}
		seen[container] = true
		container = container.Elem()
	}
	if container.Kind() != reflect.Slice && container.Kind() != reflect.Array {
		return nil, fmt.Errorf("collection %T does not contain %v", value, entity)
	}
	element := container.Elem()
	values := element == entity
	pointers := element.Kind() == reflect.Pointer && element.Elem() == entity && element.ConvertibleTo(pointer)
	if !values && !pointers {
		return nil, fmt.Errorf("collection %T has element %v, expected %v or %v", value, element, entity, pointer)
	}
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil, nil
		}
		v = v.Elem()
	}
	if v.Kind() == reflect.Slice && v.IsNil() {
		return nil, nil
	}
	if v.Kind() == reflect.Array && !v.CanAddr() {
		copy := reflect.New(v.Type()).Elem()
		copy.Set(v)
		v = copy
	}
	result := make([]*T, v.Len())
	for i := range result {
		row := v.Index(i)
		if values {
			result[i] = row.Addr().Interface().(*T)
		} else {
			result[i] = row.Convert(pointer).Interface().(*T)
		}
	}
	return result, nil
}

package shape

import (
	"fmt"
	"reflect"
)

// FieldValue associates a compiled field accessor with an assignable value.
// Values are not cloned; callers choose their own ownership/detachment policy.
type FieldValue struct {
	Accessor *Accessor
	Value    any
}

// AssignFields validates every write before publishing any of them. Existing
// pointer holders retain their identities; only missing holders are allocated.
// The caller must exclusively own target during assignment. Overlapping field
// paths are rejected rather than making their order part of the contract.
func (Runtime) AssignFields(target any, fields ...FieldValue) error {
	root := reflect.ValueOf(target)
	rootTypes := map[reflect.Type]bool{}
	for root.IsValid() && root.Kind() == reflect.Pointer {
		if rootTypes[root.Type()] {
			return fmt.Errorf("field target has a recursive pointer type")
		}
		rootTypes[root.Type()] = true
		if root.IsNil() {
			return fmt.Errorf("field target is nil")
		}
		root = root.Elem()
	}
	if !root.IsValid() || root.Kind() != reflect.Struct || !root.CanSet() {
		return fmt.Errorf("field target must be an addressed struct")
	}
	type write struct {
		destination, source reflect.Value
		ancestors           []reflect.Value
	}
	sameLocation := func(a, b reflect.Value) bool {
		return a == b || (a.Type() == b.Type() && a.CanAddr() && b.CanAddr() && a.Addr().Pointer() == b.Addr().Pointer())
	}
	overlapsHolder := func(destination, ancestor reflect.Value) bool {
		if sameLocation(destination, ancestor) {
			return true
		}
		return destination.Kind() == reflect.Pointer && !destination.IsNil() && sameLocation(destination.Elem(), ancestor)
	}
	var allocations, writes []write
	pending := map[reflect.Value]reflect.Value{}
	for i, field := range fields {
		a := field.Accessor
		if a == nil || a.owner != root.Type() || len(a.steps) != 0 || len(a.field.Index) == 0 {
			return fmt.Errorf("field %d has no matching direct accessor", i)
		}
		for _, prior := range fields[:i] {
			left, right := a.field.Index, prior.Accessor.field.Index
			limit := len(left)
			if len(right) < limit {
				limit = len(right)
			}
			overlap := true
			for index := 0; index < limit; index++ {
				if left[index] != right[index] {
					overlap = false
					break
				}
			}
			if overlap {
				return fmt.Errorf("field paths %s and %s overlap", prior.Accessor.field.Name, a.field.Name)
			}
		}
		source := reflect.Zero(a.field.Type)
		if field.Value != nil {
			source = reflect.ValueOf(field.Value)
			if !source.Type().AssignableTo(a.field.Type) {
				return fmt.Errorf("cannot assign %s to field %s of type %s", source.Type(), a.field.Name, a.field.Type)
			}
		}
		value := root
		var ancestors []reflect.Value
		for _, index := range a.field.Index {
			for value.Kind() == reflect.Pointer {
				ancestors = append(ancestors, value)
				if value.IsNil() {
					if !value.CanSet() {
						return fmt.Errorf("field path %s is not writable", a.field.Name)
					}
					next, ok := pending[value]
					if !ok {
						next = reflect.New(value.Type().Elem())
						if next.Type() != value.Type() {
							if !next.Type().ConvertibleTo(value.Type()) {
								return fmt.Errorf("field holder %s cannot be allocated", value.Type())
							}
							next = next.Convert(value.Type())
						}
						pending[value] = next
						allocations = append(allocations, write{destination: value, source: next})
					}
					value = next
				}
				value = value.Elem()
			}
			if value.Kind() != reflect.Struct || index < 0 || index >= value.NumField() {
				return fmt.Errorf("invalid compiled field path %s", a.field.Name)
			}
			ancestors = append(ancestors, value)
			value = value.Field(index)
		}
		if !value.CanSet() || !value.CanInterface() {
			return fmt.Errorf("field %s is not writable", a.field.Name)
		}
		// Aliased holders can make different selectors identify the same leaf.
		for _, prior := range writes {
			if sameLocation(prior.destination, value) {
				return fmt.Errorf("field %s aliases another assignment", a.field.Name)
			}
			for _, ancestor := range ancestors {
				if overlapsHolder(prior.destination, ancestor) {
					return fmt.Errorf("field %s overlaps an aliased holder", a.field.Name)
				}
			}
			for _, ancestor := range prior.ancestors {
				if overlapsHolder(value, ancestor) {
					return fmt.Errorf("field %s overlaps an aliased holder", a.field.Name)
				}
			}
		}
		writes = append(writes, write{destination: value, source: source, ancestors: ancestors})
	}
	// Every operation below is a prevalidated reflect assignment. Attaching
	// newly allocated holders last keeps them private until their values exist.
	for _, item := range writes {
		item.destination.Set(item.source)
	}
	for i := len(allocations) - 1; i >= 0; i-- {
		allocations[i].destination.Set(allocations[i].source)
	}
	return nil
}

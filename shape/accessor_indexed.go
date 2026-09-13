package shape

import (
	"fmt"
	"reflect"
	"strings"
)

type indexedField struct {
	collections int
	index       []int
}

// IndexedAccessor compiles a field path that can cross slice/array elements.
// Each collection crossed before the next field consumes one GetAt index;
// remaining indexes address elements of the final field.
func (t *Type) IndexedAccessor(path string) (*Accessor, error) {
	if t == nil || t.descriptor == nil || t.descriptor.Type == nil {
		return nil, fmt.Errorf("indexed accessor requires a linked type")
	}
	owner := (Runtime{}).Indirect(t.descriptor.Type)
	if owner.Kind() != reflect.Struct {
		return nil, fmt.Errorf("indexed accessor owner must be a struct")
	}
	result := &Accessor{owner: owner}
	current := owner
	for _, name := range strings.Split(strings.ReplaceAll(path, "/", "."), ".") {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("indexed field path contains an empty segment")
		}
		step := indexedField{}
		current = (Runtime{}).Indirect(current)
		for current.Kind() == reflect.Slice || current.Kind() == reflect.Array {
			step.collections++
			current = (Runtime{}).Indirect(current.Elem())
		}
		field, err := Linked(current).StructField(name)
		if err != nil {
			return nil, err
		}
		if !field.IsExported() {
			return nil, fmt.Errorf("field %s is not exported", name)
		}
		step.index = append([]int(nil), field.Index...)
		result.steps = append(result.steps, step)
		result.field = field
		current = field.Type
	}
	if len(result.steps) == 0 {
		return nil, fmt.Errorf("indexed field path is required")
	}
	return result, nil
}

// GetAt returns an addressable source value whenever the supplied source is
// addressable. Nil ancestors yield an invalid Value without allocating them.
func (a *Accessor) GetAt(target any, indexes ...int) (reflect.Value, error) {
	if a == nil {
		return reflect.Value{}, fmt.Errorf("field accessor is required")
	}
	if len(a.steps) == 0 {
		if len(indexes) > 0 {
			return reflect.Value{}, fmt.Errorf("field accessor is not indexed")
		}
		return a.Get(target)
	}
	value := reflect.ValueOf(target)
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}, nil
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Type() != a.owner {
		return reflect.Value{}, fmt.Errorf("indexed field target must be %s", a.owner)
	}
	position := 0
	indexValue := func() error {
		for value.Kind() == reflect.Pointer {
			if value.IsNil() {
				value = reflect.Value{}
				return nil
			}
			value = value.Elem()
		}
		if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
			return fmt.Errorf("extra index for non-collection field")
		}
		if position >= len(indexes) {
			return fmt.Errorf("missing collection index")
		}
		index := indexes[position]
		position++
		if index < 0 || index >= value.Len() {
			return fmt.Errorf("collection index %d out of range", index)
		}
		value = value.Index(index)
		return nil
	}
	for _, step := range a.steps {
		for i := 0; i < step.collections; i++ {
			if err := indexValue(); err != nil {
				return reflect.Value{}, err
			}
			if !value.IsValid() {
				return value, nil
			}
		}
		for _, index := range step.index {
			for value.Kind() == reflect.Pointer {
				if value.IsNil() {
					return reflect.Value{}, nil
				}
				value = value.Elem()
			}
			if value.Kind() != reflect.Struct || index >= value.NumField() {
				return reflect.Value{}, fmt.Errorf("invalid compiled indexed field")
			}
			value = value.Field(index)
		}
	}
	for position < len(indexes) {
		if err := indexValue(); err != nil {
			return reflect.Value{}, err
		}
		if !value.IsValid() {
			return value, nil
		}
	}
	if !value.CanInterface() {
		return reflect.Value{}, fmt.Errorf("indexed field is not accessible")
	}
	return value, nil
}

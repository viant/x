package shape

import (
	"fmt"
	"reflect"
)

// Accessor is an immutable compiled field path with checked runtime access.
type Accessor struct {
	owner reflect.Type
	field reflect.StructField
	steps []indexedField
}

func (t *Type) Accessor(path string) (*Accessor, error) {
	if t == nil || t.descriptor == nil || t.descriptor.Type == nil {
		return nil, fmt.Errorf("runtime field accessor requires a linked type")
	}
	field, err := t.StructField(path)
	if err != nil {
		return nil, err
	}
	if !field.IsExported() {
		return nil, fmt.Errorf("field %s is not exported", path)
	}
	field.Index = append([]int(nil), field.Index...)
	return &Accessor{owner: (Runtime{}).Indirect(t.descriptor.Type), field: field}, nil
}

func (a *Accessor) Type() reflect.Type {
	if a == nil {
		return nil
	}
	return a.field.Type
}
func (a *Accessor) Get(target any) (reflect.Value, error) { return a.value(target, false) }

func (a *Accessor) Set(target any, value any) error {
	if a == nil {
		return fmt.Errorf("field accessor is required")
	}
	var source reflect.Value
	if value != nil {
		source = reflect.ValueOf(value)
		if !source.Type().AssignableTo(a.field.Type) {
			return fmt.Errorf("cannot assign %s to field %s of type %s", source.Type(), a.field.Name, a.field.Type)
		}
	}
	destination, err := a.value(target, true)
	if err != nil {
		return err
	}
	if !destination.CanSet() {
		return fmt.Errorf("field %s is not writable", a.field.Name)
	}
	if value == nil {
		destination.SetZero()
	} else {
		destination.Set(source)
	}
	return nil
}

func (a *Accessor) value(target any, allocate bool) (reflect.Value, error) {
	if a != nil && len(a.steps) > 0 {
		if allocate {
			return reflect.Value{}, fmt.Errorf("indexed field assignment requires an addressed value")
		}
		return a.GetAt(target)
	}
	if a == nil {
		return reflect.Value{}, fmt.Errorf("field accessor is required")
	}
	value := reflect.ValueOf(target)
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}, fmt.Errorf("field target is nil")
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Type() != a.owner {
		return reflect.Value{}, fmt.Errorf("field target must be %s", a.owner)
	}
	for _, index := range a.field.Index {
		for value.Kind() == reflect.Pointer {
			if value.IsNil() {
				if !allocate {
					return reflect.Zero(a.field.Type), nil
				}
				if !value.CanSet() {
					return reflect.Value{}, fmt.Errorf("field path %s is not writable", a.field.Name)
				}
				value.Set(reflect.New(value.Type().Elem()))
			}
			value = value.Elem()
		}
		if value.Kind() != reflect.Struct || index < 0 || index >= value.NumField() {
			return reflect.Value{}, fmt.Errorf("invalid compiled field path %s", a.field.Name)
		}
		value = value.Field(index)
	}
	if !value.CanInterface() {
		return reflect.Value{}, fmt.Errorf("field %s is not accessible", a.field.Name)
	}
	return value, nil
}

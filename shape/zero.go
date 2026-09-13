package shape

import (
	"fmt"
	"reflect"
)

// WithZeroFields returns the same runtime type with selected canonical field
// paths zeroed. Root and modified struct/pointer branches are copied; untouched
// branches remain shared. No mutation reaches the supplied value.
func (r Runtime) WithZeroFields(source any, paths ...string) (any, error) {
	value := reflect.ValueOf(source)
	if !value.IsValid() {
		return nil, fmt.Errorf("source value is required")
	}
	typeOf := r.Indirect(value.Type())
	if typeOf.Kind() != reflect.Struct {
		return nil, fmt.Errorf("source must be a struct or struct pointer, got %T", source)
	}
	for _, path := range paths {
		field, err := Linked(value.Type()).StructField(path)
		if err != nil {
			return nil, err
		}
		value, err = r.zeroPath(value, field.Index)
		if err != nil {
			return nil, err
		}
	}
	if len(paths) == 0 {
		return r.zeroPathRoot(value).Interface(), nil
	}
	return value.Interface(), nil
}

func (r Runtime) zeroPathRoot(value reflect.Value) reflect.Value {
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		copy := reflect.New(value.Type().Elem())
		copy.Elem().Set(r.zeroPathRoot(value.Elem()))
		return copy
	}
	copy := reflect.New(value.Type()).Elem()
	copy.Set(value)
	return copy
}

func (r Runtime) zeroPath(value reflect.Value, indexes []int) (reflect.Value, error) {
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Zero(value.Type()), nil
		}
		child, err := r.zeroPath(value.Elem(), indexes)
		if err != nil {
			return reflect.Value{}, err
		}
		copy := reflect.New(value.Type().Elem())
		copy.Elem().Set(child)
		return copy, nil
	}
	if value.Kind() != reflect.Struct || len(indexes) == 0 {
		return reflect.Value{}, fmt.Errorf("invalid field path traversal")
	}
	copy := reflect.New(value.Type()).Elem()
	copy.Set(value)
	field := copy.Field(indexes[0])
	if !field.CanSet() {
		return reflect.Value{}, fmt.Errorf("field %s is not writable", value.Type().Field(indexes[0]).Name)
	}
	if len(indexes) == 1 {
		field.Set(reflect.Zero(field.Type()))
		return copy, nil
	}
	child, err := r.zeroPath(field, indexes[1:])
	if err != nil {
		return reflect.Value{}, err
	}
	field.Set(child)
	return copy, nil
}

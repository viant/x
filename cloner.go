package x

import (
	"fmt"
	"reflect"

	"github.com/viant/x/syntetic/model"
)

// Cloner detaches mutable descriptors and synthetic AST payloads. Runtime Go
// types are immutable and retain their exact identity.
type Cloner struct{}

// Registry snapshots types and compiled functions under one source read lock.
// Listeners are not copied or invoked. Callable closures retain their identity;
// callers must not capture mutable generation state in shared exports.
func (c Cloner) Registry(source *Registry) (*Registry, error) {
	if source == nil {
		return nil, fmt.Errorf("registry is required")
	}
	source.mux.RLock()
	defer source.mux.RUnlock()
	result := NewRegistry(WithRegistryScn(source.scn))
	for key, typ := range source.types {
		copy, err := c.Type(typ)
		if err != nil {
			return nil, err
		}
		result.types[key] = copy
	}
	result.functions = make(map[string]*Function, len(source.functions))
	for key, function := range source.functions {
		copy := *function
		result.functions[key] = &copy
	}
	return result, nil
}

func (Cloner) Type(source *Type) (*Type, error) {
	if source == nil {
		return nil, nil
	}
	synthetic, err := (Cloner{}).Synthetic(source.SynteticType)
	if err != nil {
		return nil, err
	}
	return &Type{Type: source.Type, PkgPath: source.PkgPath, Location: source.Location, Name: source.Name, Definition: source.Definition, Scn: source.Scn, Force: source.Force, SynteticType: synthetic}, nil
}

func (Cloner) Synthetic(source *model.Type) (*model.Type, error) {
	if source == nil {
		return nil, nil
	}
	state := cloneState{seen: map[cloneIdentity]reflect.Value{}}
	value, err := state.value(reflect.ValueOf(source))
	if err != nil {
		return nil, err
	}
	return value.Interface().(*model.Type), nil
}

type cloneIdentity struct {
	kind    reflect.Kind
	typeOf  reflect.Type
	pointer uintptr
	length  int
}
type cloneState struct {
	seen map[cloneIdentity]reflect.Value
}

func (s *cloneState) value(source reflect.Value) (reflect.Value, error) {
	if !source.IsValid() {
		return source, nil
	}
	if source.CanInterface() {
		if _, ok := source.Interface().(reflect.Type); ok {
			return source, nil
		}
	}
	result := reflect.New(source.Type()).Elem()
	switch source.Kind() {
	case reflect.Interface:
		if source.IsNil() {
			return result, nil
		}
		value, err := s.value(source.Elem())
		if err != nil {
			return reflect.Value{}, err
		}
		result.Set(value)
		return result, nil
	case reflect.Pointer, reflect.Map, reflect.Slice:
		if source.IsNil() {
			return result, nil
		}
		identity := cloneIdentity{kind: source.Kind(), typeOf: source.Type(), pointer: uintptr(source.UnsafePointer())}
		if source.Kind() == reflect.Slice {
			identity.length = source.Len()
		}
		if previous, ok := s.seen[identity]; ok {
			return previous, nil
		}
		switch source.Kind() {
		case reflect.Pointer:
			result = reflect.New(source.Type().Elem())
			s.seen[identity] = result
			value, err := s.value(source.Elem())
			if err != nil {
				return reflect.Value{}, err
			}
			result.Elem().Set(value)
		case reflect.Map:
			result = reflect.MakeMapWithSize(source.Type(), source.Len())
			s.seen[identity] = result
			iterator := source.MapRange()
			for iterator.Next() {
				key, err := s.value(iterator.Key())
				if err != nil {
					return reflect.Value{}, err
				}
				value, err := s.value(iterator.Value())
				if err != nil {
					return reflect.Value{}, err
				}
				result.SetMapIndex(key, value)
			}
		case reflect.Slice:
			result = reflect.MakeSlice(source.Type(), source.Len(), source.Len())
			s.seen[identity] = result
			for i := 0; i < source.Len(); i++ {
				value, err := s.value(source.Index(i))
				if err != nil {
					return reflect.Value{}, err
				}
				result.Index(i).Set(value)
			}
		}
		return result, nil
	case reflect.Struct:
		for i := 0; i < source.NumField(); i++ {
			field := source.Type().Field(i)
			if field.PkgPath != "" {
				// Render caches are derived state; copied ASTs must render afresh.
				if source.Type() == reflect.TypeOf(model.Type{}) && field.Name == "bodyCache" {
					continue
				}
				return reflect.Value{}, fmt.Errorf("cannot clone unexported synthetic field %s.%s", source.Type(), field.Name)
			}
			value, err := s.value(source.Field(i))
			if err != nil {
				return reflect.Value{}, err
			}
			result.Field(i).Set(value)
		}
		return result, nil
	case reflect.Array:
		for i := 0; i < source.Len(); i++ {
			value, err := s.value(source.Index(i))
			if err != nil {
				return reflect.Value{}, err
			}
			result.Index(i).Set(value)
		}
		return result, nil
	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
		if !source.IsNil() {
			return reflect.Value{}, fmt.Errorf("unsupported mutable synthetic payload %s", source.Type())
		}
		return result, nil
	default:
		result.Set(source)
		return result, nil
	}
}

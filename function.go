package x

import (
	"fmt"
	"go/token"
	"golang.org/x/mod/module"
	"reflect"
)

// Function is an immutable compiled callable export. It carries native Go type
// identity; it does not compile source or discover functions by package name.
// The registering application owns any mutable state captured by the function.
type Function struct {
	pkgPath, name string
	value         reflect.Value
}

func NewFunction(packagePath, name string, value any) (*Function, error) {
	if err := module.CheckImportPath(packagePath); err != nil {
		return nil, fmt.Errorf("function package import path: %w", err)
	}
	if !token.IsIdentifier(name) || name == "_" {
		return nil, fmt.Errorf("function name %q is invalid", name)
	}
	actual := reflect.ValueOf(value)
	if !actual.IsValid() || actual.Kind() != reflect.Func || actual.IsNil() {
		return nil, fmt.Errorf("function %s.%s requires a non-nil compiled function", packagePath, name)
	}
	return &Function{pkgPath: packagePath, name: name, value: actual}, nil
}

func (f *Function) Key() string {
	if f == nil {
		return ""
	}
	return f.pkgPath + "." + f.name
}
func (f *Function) Name() string {
	if f == nil {
		return ""
	}
	return f.name
}
func (f *Function) PackagePath() string {
	if f == nil {
		return ""
	}
	return f.pkgPath
}
func (f *Function) Type() reflect.Type {
	if f == nil || !f.value.IsValid() {
		return nil
	}
	return f.value.Type()
}

// Call invokes a compiled function with assignable native Go arguments. For a
// variadic function pass trailing arguments individually, as in an ordinary
// Go call. Return values, including an error result, are returned unchanged;
// interpreting a particular factory signature belongs to the caller.
func (f *Function) Call(args ...any) (results []any, err error) {
	if f == nil || !f.value.IsValid() {
		return nil, fmt.Errorf("compiled function is required")
	}
	typ := f.Type()
	minimum := typ.NumIn()
	if typ.IsVariadic() {
		minimum--
	}
	if len(args) < minimum || !typ.IsVariadic() && len(args) != minimum {
		return nil, fmt.Errorf("function %s argument count mismatch", f.Key())
	}
	values := make([]reflect.Value, len(args))
	for index, arg := range args {
		position := index
		variadic := typ.IsVariadic() && index >= minimum
		if variadic {
			position = minimum
		}
		target := typ.In(position)
		if variadic {
			target = target.Elem()
		}
		if arg == nil {
			switch target.Kind() {
			case reflect.Interface, reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.UnsafePointer:
				values[index] = reflect.Zero(target)
			default:
				return nil, fmt.Errorf("function %s argument %d cannot be nil", f.Key(), index)
			}
			continue
		}
		values[index] = reflect.ValueOf(arg)
		if !values[index].Type().AssignableTo(target) {
			return nil, fmt.Errorf("function %s argument %d has type %s, want %s", f.Key(), index, values[index].Type(), target)
		}
	}
	completed := false
	defer func() {
		if !completed {
			results = nil
			err = &FunctionPanicError{Key: f.Key(), Value: recover()}
		}
	}()
	returned := f.value.Call(values)
	results = make([]any, len(returned))
	for index, value := range returned {
		results[index] = value.Interface()
	}
	completed = true
	return results, nil
}

// FunctionPanicError distinguishes a compiled function panic from validation
// errors and preserves an error-valued panic for errors.Is/errors.As.
type FunctionPanicError struct {
	Key   string
	Value any
}

func (e *FunctionPanicError) Error() string {
	return fmt.Sprintf("function %s panicked: %v", e.Key, e.Value)
}
func (e *FunctionPanicError) Unwrap() error { cause, _ := e.Value.(error); return cause }

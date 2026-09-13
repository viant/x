package shape

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strconv"

	"github.com/viant/x"
	loader "github.com/viant/x/loader/xreflect"
)

type Runtime struct {
	Imports map[string]string
	Lookup  func(string) (reflect.Type, error)
}
type RuntimeField struct {
	Name, TypeExpr string
	Type           reflect.Type
	Tag            reflect.StructTag
	Anonymous      bool
	PkgPath        string
}

func (Runtime) Indirect(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}
func (Runtime) Pointer(t reflect.Type) reflect.Type {
	if t == nil {
		return nil
	}
	return reflect.PointerTo(t)
}

func (r Runtime) Struct(fields []RuntimeField) (result reflect.Type, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			err = fmt.Errorf("construct runtime struct: %v", recovered)
		}
	}()
	seen := map[string]bool{}
	resolved := make([]reflect.StructField, len(fields))
	for i, field := range fields {
		if !token.IsIdentifier(field.Name) || seen[field.Name] {
			return nil, fmt.Errorf("invalid or duplicate runtime field %q", field.Name)
		}
		seen[field.Name] = true
		typeOf := field.Type
		if typeOf == nil {
			typeOf, err = r.Type(field.TypeExpr)
			if err != nil {
				return nil, fmt.Errorf("field %s: %w", field.Name, err)
			}
		}
		if typeOf == nil {
			return nil, fmt.Errorf("field %s has no runtime type", field.Name)
		}
		resolved[i] = reflect.StructField{Name: field.Name, Type: typeOf, Tag: field.Tag, Anonymous: field.Anonymous, PkgPath: field.PkgPath}
	}
	return reflect.StructOf(resolved), nil
}

func (r Runtime) Type(source string) (result reflect.Type, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			err = fmt.Errorf("construct runtime type %q: %v", source, recovered)
		}
	}()
	expression, err := (Resolver{}).parse(source)
	if err != nil {
		return nil, err
	}
	return r.expression(expression)
}

func (r Runtime) expression(expression ast.Expr) (reflect.Type, error) {
	switch value := expression.(type) {
	case *ast.Ident:
		if result := runtimeBuiltin(value.Name); result != nil {
			return result, nil
		}
		return r.named(value.Name)
	case *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		return r.named(rendered(expression))
	case *ast.StarExpr:
		element, err := r.expression(value.X)
		if err != nil {
			return nil, err
		}
		return reflect.PointerTo(element), nil
	case *ast.ArrayType:
		element, err := r.expression(value.Elt)
		if err != nil {
			return nil, err
		}
		if value.Len == nil {
			return reflect.SliceOf(element), nil
		}
		length, err := strconv.Atoi(rendered(value.Len))
		if err != nil || length < 0 {
			return nil, fmt.Errorf("array length must be a non-negative integer")
		}
		return reflect.ArrayOf(length, element), nil
	case *ast.MapType:
		key, err := r.expression(value.Key)
		if err != nil {
			return nil, err
		}
		element, err := r.expression(value.Value)
		if err != nil {
			return nil, err
		}
		if !key.Comparable() {
			return nil, fmt.Errorf("map key %s is not comparable", key)
		}
		return reflect.MapOf(key, element), nil
	case *ast.ChanType:
		element, err := r.expression(value.Value)
		if err != nil {
			return nil, err
		}
		direction := reflect.BothDir
		if value.Dir == ast.SEND {
			direction = reflect.SendDir
		} else if value.Dir == ast.RECV {
			direction = reflect.RecvDir
		}
		return reflect.ChanOf(direction, element), nil
	case *ast.ParenExpr:
		return r.expression(value.X)
	case *ast.StructType:
		var fields []RuntimeField
		if value.Fields != nil {
			for _, field := range value.Fields.List {
				typeOf, err := r.expression(field.Type)
				if err != nil {
					return nil, err
				}
				tag := ""
				if field.Tag != nil {
					tag, err = strconv.Unquote(field.Tag.Value)
					if err != nil {
						return nil, err
					}
				}
				if len(field.Names) == 0 {
					base := typeOf
					if base.Kind() == reflect.Pointer {
						base = base.Elem()
					}
					if base.Name() == "" {
						return nil, fmt.Errorf("embedded field must have a named type")
					}
					pkgPath := ""
					if !token.IsExported(base.Name()) {
						pkgPath = base.PkgPath()
					}
					fields = append(fields, RuntimeField{Name: base.Name(), Type: typeOf, Tag: reflect.StructTag(tag), Anonymous: true, PkgPath: pkgPath})
					continue
				}
				for _, name := range field.Names {
					fields = append(fields, RuntimeField{Name: name.Name, Type: typeOf, Tag: reflect.StructTag(tag)})
				}
			}
		}
		return r.Struct(fields)
	case *ast.InterfaceType:
		if value.Methods == nil || len(value.Methods.List) == 0 {
			return reflect.TypeOf((*any)(nil)).Elem(), nil
		}
		return r.named(rendered(value))
	case *ast.FuncType:
		inputs, variadic, err := r.parameters(value.Params, true)
		if err != nil {
			return nil, err
		}
		outputs, _, err := r.parameters(value.Results, false)
		if err != nil {
			return nil, err
		}
		return reflect.FuncOf(inputs, outputs, variadic), nil
	default:
		return nil, fmt.Errorf("unsupported runtime type expression %T", expression)
	}
}

func (r Runtime) named(source string) (reflect.Type, error) {
	if r.Lookup == nil {
		return nil, fmt.Errorf("type %q requires a runtime lookup", source)
	}
	canonical, err := (Resolver{Imports: r.Imports}).Canonical(source)
	if err != nil {
		return nil, err
	}
	result, err := r.Lookup(canonical)
	if err != nil {
		return nil, err
	}
	if result == nil && source != canonical {
		result, err = r.Lookup(source)
		if err != nil {
			return nil, err
		}
	}
	if result == nil {
		return nil, fmt.Errorf("runtime type %q was not resolved", source)
	}
	return result, nil
}

func (r Runtime) parameters(fields *ast.FieldList, allowVariadic bool) ([]reflect.Type, bool, error) {
	var result []reflect.Type
	variadic := false
	if fields == nil {
		return result, false, nil
	}
	for i, field := range fields.List {
		expression := field.Type
		if ellipsis, ok := expression.(*ast.Ellipsis); ok {
			if !allowVariadic || i != len(fields.List)-1 {
				return nil, false, fmt.Errorf("variadic argument must be the final input")
			}
			expression = &ast.ArrayType{Elt: ellipsis.Elt}
			variadic = true
		}
		typeOf, err := r.expression(expression)
		if err != nil {
			return nil, false, err
		}
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for j := 0; j < count; j++ {
			result = append(result, typeOf)
		}
	}
	return result, variadic, nil
}

func (Runtime) Synthetic(packagePath, name string, typeOf reflect.Type) (*x.Type, error) {
	if !token.IsIdentifier(name) || !token.IsExported(name) {
		return nil, fmt.Errorf("synthetic type name %q must be exported", name)
	}
	typeDescriptor, err := loader.BuildType(typeOf, loader.WithPackagePath(packagePath), loader.WithNamePolicy(func(reflect.Type) (string, bool) { return name, false }))
	if err != nil {
		return nil, err
	}
	return &x.Type{Name: name, PkgPath: packagePath, Type: typeOf, SynteticType: typeDescriptor}, nil
}

func runtimeBuiltin(name string) reflect.Type {
	switch name {
	case "any":
		return reflect.TypeOf((*any)(nil)).Elem()
	case "error":
		return reflect.TypeOf((*error)(nil)).Elem()
	case "bool":
		return reflect.TypeOf(false)
	case "string":
		return reflect.TypeOf("")
	case "int":
		return reflect.TypeOf(int(0))
	case "int8":
		return reflect.TypeOf(int8(0))
	case "int16":
		return reflect.TypeOf(int16(0))
	case "int32", "rune":
		return reflect.TypeOf(int32(0))
	case "int64":
		return reflect.TypeOf(int64(0))
	case "uint":
		return reflect.TypeOf(uint(0))
	case "uint8", "byte":
		return reflect.TypeOf(uint8(0))
	case "uint16":
		return reflect.TypeOf(uint16(0))
	case "uint32":
		return reflect.TypeOf(uint32(0))
	case "uint64":
		return reflect.TypeOf(uint64(0))
	case "uintptr":
		return reflect.TypeOf(uintptr(0))
	case "float32":
		return reflect.TypeOf(float32(0))
	case "float64":
		return reflect.TypeOf(float64(0))
	case "complex64":
		return reflect.TypeOf(complex64(0))
	case "complex128":
		return reflect.TypeOf(complex128(0))
	}
	return nil
}

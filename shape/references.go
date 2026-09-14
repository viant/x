package shape

import (
	"fmt"
	"go/ast"
	"sort"
)

// References returns canonical named type references in a type expression,
// including generic arguments, container elements and structural fields.
func (r Resolver) References(source string) ([]string, error) {
	expression, err := r.parse(source)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var walk func(ast.Expr) error
	var fields func(*ast.FieldList) error
	fields = func(list *ast.FieldList) error {
		if list != nil {
			for _, f := range list.List {
				if err := walk(f.Type); err != nil {
					return err
				}
			}
		}
		return nil
	}
	walk = func(e ast.Expr) error {
		switch v := e.(type) {
		case *ast.Ident:
			if builtin(v.Name) {
				return nil
			}
			key, err := r.Canonical(v)
			if err != nil {
				return err
			}
			seen[key] = true
		case *ast.SelectorExpr:
			key, err := r.Canonical(v)
			if err != nil {
				return err
			}
			seen[key] = true
		case *ast.IndexExpr:
			key, err := r.Canonical(v)
			if err != nil {
				return err
			}
			seen[key] = true
			return walk(v.Index)
		case *ast.IndexListExpr:
			key, err := r.Canonical(v)
			if err != nil {
				return err
			}
			seen[key] = true
			for _, arg := range v.Indices {
				if err := walk(arg); err != nil {
					return err
				}
			}
		case *ast.StarExpr:
			return walk(v.X)
		case *ast.ArrayType:
			return walk(v.Elt)
		case *ast.MapType:
			if err := walk(v.Key); err != nil {
				return err
			}
			return walk(v.Value)
		case *ast.ChanType:
			return walk(v.Value)
		case *ast.ParenExpr:
			return walk(v.X)
		case *ast.Ellipsis:
			return walk(v.Elt)
		case *ast.StructType:
			return fields(v.Fields)
		case *ast.InterfaceType:
			return fields(v.Methods)
		case *ast.FuncType:
			if err := fields(v.Params); err != nil {
				return err
			}
			return fields(v.Results)
		case *ast.UnaryExpr:
			return walk(v.X)
		case *ast.BinaryExpr:
			if err := walk(v.X); err != nil {
				return err
			}
			return walk(v.Y)
		default:
			return fmt.Errorf("unsupported reference expression %T", e)
		}
		return nil
	}
	if err := walk(expression); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(seen))
	for key := range seen {
		result = append(result, key)
	}
	sort.Strings(result)
	return result, nil
}

// References enumerates direct structural dependencies of a descriptor through
// the same import authority as field/type resolution. Methods are linked with
// their owning compiled type and do not create structural catalog entries.
func (t *Type) References() ([]string, error) {
	if t == nil || t.descriptor == nil {
		return nil, fmt.Errorf("type descriptor is required")
	}
	if t.descriptor.SynteticType == nil {
		return nil, fmt.Errorf("source descriptor is required")
	}
	declaration := t.descriptor.SynteticType
	resolver := Resolver{Package: declaration.PkgPath, Imports: map[string]string{}}
	for alias, imp := range declaration.Imports {
		resolver.Imports[alias] = imp.Path
	}
	return resolver.References(declaration.Body())
}

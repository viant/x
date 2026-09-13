package shape

import (
	"fmt"
	"go/ast"
	"go/token"
	"path"
	"reflect"
	"sort"

	model "github.com/viant/x/syntetic/model"
)

// Method is a receiver-free exported method signature. Types use full package
// identities and parameter names are intentionally not part of the contract.
type Method struct {
	Name       string
	Parameters []string
	Results    []string
	Variadic   bool
}

// Methods returns the exported method signatures available to the requested
// receiver. A pointer receiver includes value-receiver methods. Returned slices
// do not alias the descriptor. Synthetic declarations must carry method metadata.
func (t *Type) Methods(pointer bool) ([]Method, error) {
	if t != nil && t.descriptor != nil && t.descriptor.Type == nil {
		return (&methodSet{}).resolve(t, pointer)
	}
	return t.declaredMethods(pointer)
}

func (t *Type) declaredMethods(pointer bool) ([]Method, error) {
	if t == nil || t.descriptor == nil {
		return nil, fmt.Errorf("method receiver type is required")
	}
	if typ := t.descriptor.Type; typ != nil {
		base := (Runtime{}).Indirect(typ)
		if pointer {
			base = reflect.PointerTo(base)
		}
		result := make([]Method, 0, base.NumMethod())
		r := Resolver{Qualifier: func(location, suggested string) string { return location }}
		for index := 0; index < base.NumMethod(); index++ {
			method := base.Method(index)
			if !method.IsExported() {
				continue
			}
			first := 1
			if base.Kind() == reflect.Interface {
				first = 0
			}
			item := Method{Name: method.Name, Variadic: method.Type.IsVariadic()}
			for i := first; i < method.Type.NumIn(); i++ {
				expression, err := r.Expression(method.Type.In(i))
				if err != nil {
					return nil, err
				}
				canonical, err := r.Canonical(expression)
				if err != nil {
					return nil, err
				}
				item.Parameters = append(item.Parameters, canonical)
			}
			for i := 0; i < method.Type.NumOut(); i++ {
				expression, err := r.Expression(method.Type.Out(i))
				if err != nil {
					return nil, err
				}
				canonical, err := r.Canonical(expression)
				if err != nil {
					return nil, err
				}
				item.Results = append(item.Results, canonical)
			}
			result = append(result, item)
		}
		return result, nil
	}
	synthetic := t.descriptor.SynteticType
	if synthetic == nil {
		return nil, fmt.Errorf("type %s has no linked or synthetic method authority", t.descriptor.Key())
	}
	imports := map[string]string{}
	for alias, item := range synthetic.Imports {
		if item == nil {
			continue
		}
		if alias == "" {
			alias = item.Alias
		}
		if alias == "" {
			alias = path.Base(item.Path)
		}
		imports[alias] = item.Path
	}
	location := synthetic.PkgPath
	if location == "" {
		location = t.descriptor.PkgPath
	}
	collector := methodCollector{resolver: Resolver{Package: location, Imports: imports}, byName: map[string]Method{}}
	for _, decl := range synthetic.MethodsAST {
		if decl != nil {
			if err := collector.add(decl.Name.Name, decl.Type); err != nil {
				return nil, err
			}
		}
	}
	if pointer {
		for _, decl := range synthetic.PtrMethodsAST {
			if decl != nil {
				if err := collector.add(decl.Name.Name, decl.Type); err != nil {
					return nil, err
				}
			}
		}
	}
	aliases := map[string]string{}
	for alias, location := range imports {
		aliases[location] = alias
	}
	sets := [][]model.Method{synthetic.Methods.Value}
	if pointer {
		sets = append(sets, synthetic.Methods.Pointer)
	}
	for _, set := range sets {
		for _, method := range set {
			if err := collector.add(method.Name, method.Type.TypeAST(location, aliases)); err != nil {
				return nil, err
			}
		}
	}
	result := make([]Method, 0, len(collector.byName))
	for _, method := range collector.byName {
		result = append(result, method)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

type methodCollector struct {
	resolver Resolver
	byName   map[string]Method
}

func (c *methodCollector) add(name string, signature *ast.FuncType) error {
	if !token.IsExported(name) {
		return nil
	}
	if signature == nil {
		return fmt.Errorf("method %s signature is missing", name)
	}
	item := Method{Name: name}
	for _, pair := range []struct {
		fields     *ast.FieldList
		target     *[]string
		parameters bool
	}{{signature.Params, &item.Parameters, true}, {signature.Results, &item.Results, false}} {
		if pair.fields == nil {
			continue
		}
		for _, field := range pair.fields.List {
			typ := field.Type
			if variadic, ok := typ.(*ast.Ellipsis); ok {
				if !pair.parameters {
					return fmt.Errorf("method %s has a variadic result", name)
				}
				item.Variadic = true
				typ = &ast.ArrayType{Elt: variadic.Elt}
			}
			canonical, err := c.resolver.Canonical(typ)
			if err != nil {
				return fmt.Errorf("method %s: %w", name, err)
			}
			count := len(field.Names)
			if count == 0 {
				count = 1
			}
			for i := 0; i < count; i++ {
				*pair.target = append(*pair.target, canonical)
			}
		}
	}
	if prior, ok := c.byName[name]; ok && !reflect.DeepEqual(prior, item) {
		return fmt.Errorf("method %s has conflicting synthetic signatures", name)
	}
	c.byName[name] = item
	return nil
}

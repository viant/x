package shape

import (
	"fmt"
	model "github.com/viant/x/syntetic/model"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path"
	"reflect"
)

// goTypeAuthority bridges canonical descriptors into one invocation-local Go
// type universe. Named identity, cycles, packages and method sets share a cache.
type goTypeAuthority struct {
	named    map[string]*types.Named
	packages map[string]*types.Package
	linked   map[reflect.Type]types.Type
	active   map[string]bool
}

func newGoTypeAuthority() *goTypeAuthority {
	return &goTypeAuthority{named: map[string]*types.Named{}, packages: map[string]*types.Package{}, linked: map[reflect.Type]types.Type{}, active: map[string]bool{}}
}
func (b *goTypeAuthority) pkg(location string) *types.Package {
	if location == "" {
		return nil
	}
	if found := b.packages[location]; found != nil {
		return found
	}
	result := types.NewPackage(location, path.Base(location))
	b.packages[location] = result
	return result
}
func (b *goTypeAuthority) expression(r Resolver, source string, bindings map[string]types.Type) (types.Type, error) {
	expression, err := r.parse(source)
	if err != nil {
		return nil, err
	}
	return b.syntax(r, expression, bindings)
}

func (b *goTypeAuthority) syntax(r Resolver, expression ast.Expr, bindings map[string]types.Type) (types.Type, error) {
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return b.syntax(r, value.X, bindings)
	case *ast.Ident:
		if bound := bindings[value.Name]; bound != nil {
			return bound, nil
		}
		if builtin := types.Universe.Lookup(value.Name); builtin != nil {
			if _, ok := builtin.(*types.TypeName); ok {
				return builtin.Type(), nil
			}
		}
	case *ast.StarExpr:
		element, err := b.syntax(r, value.X, bindings)
		if err != nil {
			return nil, err
		}
		return types.NewPointer(element), nil
	case *ast.ArrayType:
		element, err := b.syntax(r, value.Elt, bindings)
		if err != nil {
			return nil, err
		}
		if value.Len == nil {
			return types.NewSlice(element), nil
		}
		length, err := types.Eval(token.NewFileSet(), nil, token.NoPos, rendered(value.Len))
		if err != nil {
			return nil, fmt.Errorf("array length requires constant authority: %w", err)
		}
		size, ok := constant.Int64Val(length.Value)
		if !ok || size < 0 {
			return nil, fmt.Errorf("invalid array length %s", rendered(value.Len))
		}
		return types.NewArray(element, size), nil
	case *ast.MapType:
		key, err := b.syntax(r, value.Key, bindings)
		if err != nil {
			return nil, err
		}
		element, err := b.syntax(r, value.Value, bindings)
		if err != nil {
			return nil, err
		}
		if !types.Comparable(key) {
			return nil, fmt.Errorf("map key is not comparable")
		}
		return types.NewMap(key, element), nil
	case *ast.ChanType:
		element, err := b.syntax(r, value.Value, bindings)
		if err != nil {
			return nil, err
		}
		direction := types.SendRecv
		if value.Dir == ast.SEND {
			direction = types.SendOnly
		}
		if value.Dir == ast.RECV {
			direction = types.RecvOnly
		}
		return types.NewChan(direction, element), nil
	case *ast.FuncType:
		return b.signature(r, value, bindings, nil)
	case *ast.StructType:
		fields := []*types.Var{}
		tags := []string{}
		if value.Fields != nil {
			for _, field := range value.Fields.List {
				typ, err := b.syntax(r, field.Type, bindings)
				if err != nil {
					return nil, err
				}
				names := field.Names
				anonymous := len(names) == 0
				if anonymous {
					reference, err := (Resolver{}).Reference(rendered(field.Type))
					if err != nil {
						return nil, err
					}
					names = []*ast.Ident{ast.NewIdent(reference.BaseName)}
				}
				for _, name := range names {
					fields = append(fields, types.NewField(token.NoPos, b.pkg(r.Package), name.Name, typ, anonymous))
					tag := ""
					if field.Tag != nil {
						literal := constant.MakeFromLiteral(field.Tag.Value, token.STRING, 0)
						tag = constant.StringVal(literal)
					}
					tags = append(tags, tag)
				}
			}
		}
		return types.NewStruct(fields, tags), nil
	case *ast.InterfaceType:
		methods := []*types.Func{}
		embedded := []types.Type{}
		if value.Methods != nil {
			for _, field := range value.Methods.List {
				typ, err := b.syntax(r, field.Type, bindings)
				if err != nil {
					return nil, err
				}
				if len(field.Names) == 0 {
					embedded = append(embedded, typ)
					continue
				}
				signature, ok := typ.(*types.Signature)
				if !ok {
					return nil, fmt.Errorf("interface method is not a signature")
				}
				for _, name := range field.Names {
					methods = append(methods, types.NewFunc(token.NoPos, b.pkg(r.Package), name.Name, signature))
				}
			}
		}
		return types.NewInterfaceType(methods, embedded).Complete(), nil
	case *ast.BinaryExpr:
		if value.Op != token.OR {
			return nil, fmt.Errorf("unsupported type expression %s", rendered(expression))
		}
		terms, err := b.terms(r, value, bindings)
		if err != nil {
			return nil, err
		}
		return types.NewUnion(terms), nil
	case *ast.UnaryExpr:
		if value.Op == token.TILDE {
			terms, err := b.terms(r, value, bindings)
			if err != nil {
				return nil, err
			}
			return types.NewUnion(terms), nil
		}
	}
	if len(bindings) > 0 {
		substitute := Resolver{Rewriter: func(name string) (string, error) {
			if bound := bindings[name]; bound != nil {
				return types.TypeString(bound, func(pkg *types.Package) string { return pkg.Path() }), nil
			}
			return name, nil
		}}
		rewritten, err := substitute.rewrite(expression)
		if err != nil {
			return nil, err
		}
		expression = rewritten
	}
	return b.resolve(r, rendered(expression))
}
func (b *goTypeAuthority) terms(r Resolver, expression ast.Expr, bindings map[string]types.Type) ([]*types.Term, error) {
	if union, ok := expression.(*ast.BinaryExpr); ok && union.Op == token.OR {
		left, err := b.terms(r, union.X, bindings)
		if err != nil {
			return nil, err
		}
		right, err := b.terms(r, union.Y, bindings)
		return append(left, right...), err
	}
	tilde := false
	if approximate, ok := expression.(*ast.UnaryExpr); ok && approximate.Op == token.TILDE {
		tilde = true
		expression = approximate.X
	}
	typ, err := b.syntax(r, expression, bindings)
	if err != nil {
		return nil, err
	}
	return []*types.Term{types.NewTerm(tilde, typ)}, nil
}
func (b *goTypeAuthority) tuple(r Resolver, fields *ast.FieldList, bindings map[string]types.Type) (*types.Tuple, bool, error) {
	variables := []*types.Var{}
	variadic := false
	if fields != nil {
		for _, field := range fields.List {
			expression := field.Type
			if ellipsis, ok := expression.(*ast.Ellipsis); ok {
				variadic = true
				expression = &ast.ArrayType{Elt: ellipsis.Elt}
			}
			typ, err := b.syntax(r, expression, bindings)
			if err != nil {
				return nil, false, err
			}
			count := len(field.Names)
			if count == 0 {
				count = 1
			}
			for i := 0; i < count; i++ {
				variables = append(variables, types.NewVar(token.NoPos, b.pkg(r.Package), "", typ))
			}
		}
	}
	return types.NewTuple(variables...), variadic, nil
}
func (b *goTypeAuthority) signature(r Resolver, fn *ast.FuncType, bindings map[string]types.Type, receiver *types.Var) (*types.Signature, error) {
	parameters, variadic, err := b.tuple(r, fn.Params, bindings)
	if err != nil {
		return nil, err
	}
	results, invalid, err := b.tuple(r, fn.Results, bindings)
	if err != nil {
		return nil, err
	}
	if invalid {
		return nil, fmt.Errorf("variadic result is invalid")
	}
	return types.NewSignatureType(receiver, nil, nil, parameters, results, variadic), nil
}
func (b *goTypeAuthority) resolve(r Resolver, source string) (types.Type, error) {
	resolved, err := r.Resolve(source)
	if err != nil {
		return nil, err
	}
	if resolved == nil || resolved.Descriptor == nil {
		return nil, fmt.Errorf("type %s requires supplied descriptor authority", source)
	}
	resolved.Descriptor = New(resolved.Descriptor, r.Lookup).promotionAuthority().descriptor
	if typ := resolved.Descriptor.Type; typ != nil {
		return b.reflected(typ)
	}
	if found := b.named[resolved.Identity]; found != nil {
		return found, nil
	}
	if b.active[resolved.Identity] {
		return nil, fmt.Errorf("cyclic alias or underlying type %s", resolved.Identity)
	}
	b.active[resolved.Identity] = true
	defer delete(b.active, resolved.Identity)
	descriptor := resolved.Descriptor.SynteticType
	if descriptor == nil || descriptor.TypeSpec == nil {
		return nil, fmt.Errorf("type %s requires structural authority", source)
	}
	child := r
	child.Package = resolved.Descriptor.PkgPath
	child.Imports = map[string]string{}
	for alias, item := range descriptor.Imports {
		if item != nil {
			child.Imports[alias] = item.Path
		}
	}
	if descriptor.TypeSpec.Assign.IsValid() {
		return b.syntax(child, descriptor.TypeSpec.Type, nil)
	}
	named := types.NewNamed(types.NewTypeName(token.NoPos, b.pkg(child.Package), resolved.Descriptor.Name, nil), nil, nil)
	b.named[resolved.Identity] = named
	underlying, err := b.syntax(child, descriptor.TypeSpec.Type, nil)
	if err != nil {
		return nil, err
	}
	named.SetUnderlying(underlying.Underlying())
	known := map[string]bool{}
	add := func(name string, fn *ast.FuncType, pointer bool) error {
		if known[name] {
			return nil
		}
		known[name] = true
		var receiver types.Type = named
		if pointer {
			receiver = types.NewPointer(named)
		}
		signature, err := b.signature(child.methodScope(descriptor, name), fn, nil, types.NewVar(token.NoPos, b.pkg(child.Package), "", receiver))
		if err != nil {
			return err
		}
		named.AddMethod(types.NewFunc(token.NoPos, b.pkg(child.Package), name, signature))
		return nil
	}
	for _, set := range []struct {
		declarations []*ast.FuncDecl
		pointer      bool
	}{{descriptor.MethodsAST, false}, {descriptor.PtrMethodsAST, true}} {
		for _, method := range set.declarations {
			if method != nil {
				if err := add(method.Name.Name, method.Type, set.pointer); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, set := range []struct {
		methods []model.Method
		pointer bool
	}{{descriptor.Methods.Value, false}, {descriptor.Methods.Pointer, true}} {
		for _, method := range set.methods {
			aliases := map[string]string{}
			for alias, location := range child.methodScope(descriptor, method.Name).Imports {
				aliases[location] = alias
			}
			if err := add(method.Name, method.Type.TypeAST(child.Package, aliases), set.pointer); err != nil {
				return nil, err
			}
		}
	}
	return named, nil
}

// Package shape owns structural Go type operations shared by runtime and
// source consumers. Named identity is resolved through a caller-owned lookup.
package shape

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"github.com/viant/x"
)

type Lookup func(string) (*x.Type, error)
type Resolver struct {
	Lookup    Lookup
	Imports   map[string]string
	Package   string
	Rewriter  func(string) (string, error)
	Qualifier func(packagePath, suggested string) string
}

type WrapperKind string

const (
	WrapperPointer WrapperKind = "pointer"
	WrapperSlice   WrapperKind = "slice"
	WrapperArray   WrapperKind = "array"
)

type Wrapper struct {
	Kind   WrapperKind
	Length string
}
type Reference struct {
	Qualifier, Name, BaseName string
	Arguments                 []string
	Wrappers                  []Wrapper
}

func (r Reference) QualifiedName() string {
	if r.Qualifier != "" {
		return r.Qualifier + "." + r.Name
	}
	return r.Name
}
func (r Reference) Exported() bool { return token.IsExported(r.BaseName) }

type Resolution struct {
	Identity   string
	Descriptor *x.Type
}

func rendered(expression ast.Expr) string {
	var out bytes.Buffer
	_ = format.Node(&out, token.NewFileSet(), expression)
	return out.String()
}

// parse normalizes full import-path identities into AST identifiers. The
// result is structural syntax; only Canonical may emit non-Go import paths.
func (r Resolver) parse(source string) (ast.Expr, error) {
	var normalized strings.Builder
	identities := map[string]string{}
	for i := 0; i < len(source); {
		if source[i] == '`' || source[i] == '\'' || source[i] == '"' {
			quote := source[i]
			start := i
			i++
			for i < len(source) {
				if source[i] == '\\' && quote != '`' {
					i += 2
					continue
				}
				if source[i] == quote {
					i++
					break
				}
				i++
			}
			if i > len(source) {
				i = len(source)
			}
			normalized.WriteString(source[start:i])
			continue
		}
		if unicode.IsLetter(rune(source[i])) || source[i] == '_' {
			start := i
			i++
			for i < len(source) {
				b := source[i]
				if !(unicode.IsLetter(rune(b)) || b >= '0' && b <= '9' || b == '_' || b == '.' || b == '/' || b == '-') {
					break
				}
				i++
			}
			name := source[start:i]
			if strings.Contains(name, "/") {
				placeholder := fmt.Sprintf("__shape_name_%d", len(identities))
				identities[placeholder] = name
				normalized.WriteString(placeholder)
			} else {
				normalized.WriteString(name)
			}
			continue
		}
		normalized.WriteByte(source[i])
		i++
	}
	expression, err := parser.ParseExpr(normalized.String())
	if err != nil {
		return nil, err
	}
	ast.Inspect(expression, func(n ast.Node) bool {
		if identifier, ok := n.(*ast.Ident); ok {
			if name, found := identities[identifier.Name]; found {
				identifier.Name = name
			}
		}
		return true
	})
	return expression, nil
}

func (r Resolver) Reference(source string) (Reference, error) {
	expression, err := r.parse(strings.TrimSpace(source))
	if err != nil {
		return Reference{}, err
	}
	result := Reference{}
	for {
		switch value := expression.(type) {
		case *ast.StarExpr:
			result.Wrappers = append(result.Wrappers, Wrapper{Kind: WrapperPointer})
			expression = value.X
		case *ast.ArrayType:
			kind := WrapperSlice
			length := ""
			if value.Len != nil {
				kind = WrapperArray
				length = rendered(value.Len)
			}
			result.Wrappers = append(result.Wrappers, Wrapper{Kind: kind, Length: length})
			expression = value.Elt
		case *ast.ParenExpr:
			expression = value.X
		default:
			goto named
		}
	}
named:
	base := expression
	switch generic := expression.(type) {
	case *ast.IndexExpr:
		base = generic.X
		result.Arguments = []string{rendered(generic.Index)}
	case *ast.IndexListExpr:
		base = generic.X
		for _, argument := range generic.Indices {
			result.Arguments = append(result.Arguments, rendered(argument))
		}
	}
	name := rendered(base)
	switch base.(type) {
	case *ast.Ident, *ast.SelectorExpr:
	default:
		return Reference{}, fmt.Errorf("type %q is not a named reference", source)
	}
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		result.Qualifier = name[:dot]
		name = name[dot+1:]
	}
	if !token.IsIdentifier(name) {
		return Reference{}, fmt.Errorf("invalid type name %q", name)
	}
	result.BaseName = name
	result.Name = name
	if len(result.Arguments) > 0 {
		result.Name += "[" + strings.Join(result.Arguments, ",") + "]"
	}
	return result, nil
}

func (r Resolver) Named(source string) (string, error) {
	ref, err := r.Reference(source)
	if err != nil {
		if _, parseErr := r.parse(source); parseErr != nil {
			return "", parseErr
		}
		return "", nil
	}
	return ref.QualifiedName(), nil
}
func (r Resolver) Generic(source string) (Reference, error) {
	ref, err := r.Reference(source)
	if err != nil {
		return ref, err
	}
	if len(ref.Arguments) == 0 {
		return ref, fmt.Errorf("type %q is not generic", source)
	}
	ref.Name = ref.BaseName
	return ref, nil
}
func (r Resolver) CanonicalReference(source string) (string, string, error) {
	ref, err := r.Reference(source)
	if err != nil {
		return "", "", err
	}
	return ref.Qualifier, ref.Name, nil
}

func (r Resolver) Qualifiers(source string) ([]string, error) {
	expression, err := r.parse(source)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var result []string
	ast.Inspect(expression, func(n ast.Node) bool {
		if selector, ok := n.(*ast.SelectorExpr); ok {
			if identifier, ok := selector.X.(*ast.Ident); ok && !seen[identifier.Name] {
				seen[identifier.Name] = true
				result = append(result, identifier.Name)
			}
		}
		return true
	})
	return result, nil
}

func (r Resolver) Canonical(source any) (string, error) {
	var text string
	switch value := source.(type) {
	case string:
		text = value
	case ast.Expr:
		text = rendered(value)
	default:
		return "", fmt.Errorf("canonical type requires source text or AST expression, got %T", source)
	}
	r.Rewriter = func(name string) (string, error) {
		ref, err := (Resolver{}).Reference(name)
		if err != nil {
			return "", err
		}
		qualifier := ref.Qualifier
		if mapped, ok := r.Imports[qualifier]; ok {
			qualifier = mapped
		} else if qualifier == "" && !builtin(ref.BaseName) {
			qualifier = r.Package
		}
		if qualifier != "" {
			return qualifier + "." + ref.Name, nil
		}
		return ref.Name, nil
	}
	rewritten, err := r.Rewrite(text)
	if err != nil {
		return "", err
	}
	expression, err := r.parse(rewritten)
	if err != nil {
		return "", err
	}
	return r.canonicalSyntax(expression), nil
}

func (r Resolver) canonicalSyntax(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return r.canonicalSyntax(value.X)
	case *ast.StarExpr:
		return "*" + r.canonicalSyntax(value.X)
	case *ast.ArrayType:
		length := ""
		if value.Len != nil {
			length = rendered(value.Len)
		}
		return "[" + length + "]" + r.canonicalSyntax(value.Elt)
	case *ast.MapType:
		return "map[" + r.canonicalSyntax(value.Key) + "]" + r.canonicalSyntax(value.Value)
	case *ast.IndexExpr:
		return r.canonicalSyntax(value.X) + "[" + r.canonicalSyntax(value.Index) + "]"
	case *ast.IndexListExpr:
		arguments := make([]string, len(value.Indices))
		for i, item := range value.Indices {
			arguments[i] = r.canonicalSyntax(item)
		}
		return r.canonicalSyntax(value.X) + "[" + strings.Join(arguments, ",") + "]"
	case *ast.Ellipsis:
		return "..." + r.canonicalSyntax(value.Elt)
	default:
		return rendered(expression)
	}
}

func (r Resolver) Rewrite(source string) (string, error) {
	expression, err := r.parse(source)
	if err != nil {
		return "", err
	}
	expression, err = r.rewrite(expression)
	if err != nil {
		return "", err
	}
	return rendered(expression), nil
}

func (r Resolver) rewrite(expression ast.Expr) (ast.Expr, error) {
	apply := func(name string) (ast.Expr, error) {
		if r.Rewriter == nil || builtin(name) {
			return ast.NewIdent(name), nil
		}
		replacement, err := r.Rewriter(name)
		if err != nil {
			return nil, err
		}
		return r.parse(replacement)
	}
	switch value := expression.(type) {
	case *ast.Ident:
		return apply(value.Name)
	case *ast.SelectorExpr:
		return apply(rendered(value))
	case *ast.IndexExpr:
		argument, err := r.rewrite(value.Index)
		if err != nil {
			return nil, err
		}
		value.Index = argument
		return apply(rendered(value))
	case *ast.IndexListExpr:
		for i, arg := range value.Indices {
			replacement, err := r.rewrite(arg)
			if err != nil {
				return nil, err
			}
			value.Indices[i] = replacement
		}
		return apply(rendered(value))
	case *ast.StarExpr:
		replacement, err := r.rewrite(value.X)
		value.X = replacement
		return value, err
	case *ast.ArrayType:
		replacement, err := r.rewrite(value.Elt)
		value.Elt = replacement
		return value, err
	case *ast.MapType:
		key, err := r.rewrite(value.Key)
		if err != nil {
			return nil, err
		}
		value.Key = key
		value.Value, err = r.rewrite(value.Value)
		return value, err
	case *ast.ChanType:
		replacement, err := r.rewrite(value.Value)
		value.Value = replacement
		return value, err
	case *ast.Ellipsis:
		replacement, err := r.rewrite(value.Elt)
		value.Elt = replacement
		return value, err
	case *ast.ParenExpr:
		replacement, err := r.rewrite(value.X)
		value.X = replacement
		return value, err
	case *ast.StructType:
		if err := r.rewriteFields(value.Fields); err != nil {
			return nil, err
		}
		return value, nil
	case *ast.InterfaceType:
		if err := r.rewriteFields(value.Methods); err != nil {
			return nil, err
		}
		return value, nil
	case *ast.FuncType:
		if err := r.rewriteFields(value.Params); err != nil {
			return nil, err
		}
		if err := r.rewriteFields(value.Results); err != nil {
			return nil, err
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported type expression %T", expression)
	}
}
func (r Resolver) rewriteFields(fields *ast.FieldList) error {
	if fields == nil {
		return nil
	}
	for _, field := range fields.List {
		replacement, err := r.rewrite(field.Type)
		if err != nil {
			return err
		}
		field.Type = replacement
	}
	return nil
}

func builtin(name string) bool {
	switch name {
	case "any", "interface{}", "bool", "byte", "rune", "string", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr", "float32", "float64", "complex64", "complex128", "error", "comparable":
		return true
	}
	return false
}

func (r Resolver) Resolve(source string) (*Resolution, error) {
	identity, err := r.Canonical(source)
	if err != nil {
		return nil, err
	}
	ref, err := r.Reference(identity)
	if err != nil {
		return nil, err
	}
	if r.Lookup == nil {
		return &Resolution{Identity: identity}, nil
	}
	named := ref.QualifiedName()
	descriptor, err := r.Lookup(named)
	if err != nil {
		return nil, err
	}
	if descriptor == nil && named != source {
		descriptor, err = r.Lookup(source)
		if err != nil {
			return nil, err
		}
	}
	if descriptor == nil && len(ref.Arguments) > 0 {
		base := ref.BaseName
		if ref.Qualifier != "" {
			base = ref.Qualifier + "." + base
		}
		descriptor, err = r.Lookup(base)
		if err != nil {
			return nil, err
		}
		if descriptor != nil && descriptor.SynteticType != nil && descriptor.SynteticType.TypeSpec != nil {
			descriptor, err = (x.Cloner{}).Type(descriptor)
			if err != nil {
				return nil, err
			}
			params := descriptor.SynteticType.TypeSpec.TypeParams
			if params != nil {
				names := []string{}
				for _, field := range params.List {
					for _, name := range field.Names {
						names = append(names, name.Name)
					}
				}
				if len(names) != len(ref.Arguments) {
					return nil, fmt.Errorf("type %s expects %d arguments", base, len(names))
				}
				substitute := Resolver{Rewriter: func(name string) (string, error) {
					for i, param := range names {
						if name == param {
							return ref.Arguments[i], nil
						}
					}
					return name, nil
				}}
				expr, err := substitute.rewrite(descriptor.SynteticType.TypeSpec.Type)
				if err != nil {
					return nil, err
				}
				descriptor.SynteticType.TypeSpec.Type = expr
				descriptor.SynteticType.TypeSpec.TypeParams = nil
			}
		}
	}
	return &Resolution{Identity: identity, Descriptor: descriptor}, nil
}

func (r Resolver) Expression(typeOf reflect.Type) (string, error) {
	if typeOf == nil {
		return "", fmt.Errorf("runtime type is required")
	}
	if typeOf.Name() != "" {
		if typeOf.PkgPath() == "" {
			return typeOf.Name(), nil
		}
		qualifier := path.Base(typeOf.PkgPath())
		if r.Qualifier != nil {
			qualifier = r.Qualifier(typeOf.PkgPath(), qualifier)
		}
		if qualifier == "" {
			return typeOf.Name(), nil
		}
		return qualifier + "." + typeOf.Name(), nil
	}
	child := func(t reflect.Type) (string, error) { return r.Expression(t) }
	switch typeOf.Kind() {
	case reflect.Pointer:
		elem, err := child(typeOf.Elem())
		return "*" + elem, err
	case reflect.Slice:
		elem, err := child(typeOf.Elem())
		return "[]" + elem, err
	case reflect.Array:
		elem, err := child(typeOf.Elem())
		return "[" + strconv.Itoa(typeOf.Len()) + "]" + elem, err
	case reflect.Map:
		key, err := child(typeOf.Key())
		if err != nil {
			return "", err
		}
		value, err := child(typeOf.Elem())
		return "map[" + key + "]" + value, err
	case reflect.Chan:
		elem, err := child(typeOf.Elem())
		prefix := "chan "
		if typeOf.ChanDir() == reflect.RecvDir {
			prefix = "<-chan "
		} else if typeOf.ChanDir() == reflect.SendDir {
			prefix = "chan<- "
		}
		return prefix + elem, err
	case reflect.Interface:
		if typeOf.NumMethod() == 0 {
			return "interface{}", nil
		}
		return typeOf.String(), nil
	case reflect.Struct:
		var fields []string
		for i := 0; i < typeOf.NumField(); i++ {
			field := typeOf.Field(i)
			expression, err := child(field.Type)
			if err != nil {
				return "", err
			}
			prefix := field.Name + " "
			if field.Anonymous {
				prefix = ""
			}
			tag := ""
			if field.Tag != "" {
				tag = " " + strconv.Quote(string(field.Tag))
			}
			fields = append(fields, prefix+expression+tag)
		}
		return "struct { " + strings.Join(fields, "; ") + " }", nil
	case reflect.Func:
		var args, results []string
		for i := 0; i < typeOf.NumIn(); i++ {
			arg := typeOf.In(i)
			prefix := ""
			if typeOf.IsVariadic() && i == typeOf.NumIn()-1 {
				prefix = "..."
				arg = arg.Elem()
			}
			text, err := child(arg)
			if err != nil {
				return "", err
			}
			args = append(args, prefix+text)
		}
		for i := 0; i < typeOf.NumOut(); i++ {
			text, err := child(typeOf.Out(i))
			if err != nil {
				return "", err
			}
			results = append(results, text)
		}
		result := "func(" + strings.Join(args, ", ") + ")"
		if len(results) > 0 {
			result += " (" + strings.Join(results, ", ") + ")"
		}
		return result, nil
	default:
		return "", fmt.Errorf("unsupported runtime type %s", typeOf)
	}
}

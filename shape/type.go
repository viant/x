package shape

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strconv"
	"strings"

	"github.com/viant/x"
	"github.com/viant/x/syntetic/model"
)

type Type struct {
	descriptor *x.Type
	lookup     Lookup
}
type Field struct {
	Name, TypeExpr      string
	ReflectedType       reflect.Type
	Tag                 reflect.StructTag
	Index               []int
	Anonymous, Exported bool
	PkgPath             string
}

func New(descriptor *x.Type, lookup Lookup) *Type {
	return &Type{descriptor: descriptor, lookup: lookup}
}
func Linked(typeOf reflect.Type) *Type {
	if typeOf == nil {
		return nil
	}
	base := (Runtime{}).Indirect(typeOf)
	return New(&x.Type{Type: typeOf, Name: base.Name(), PkgPath: base.PkgPath()}, nil)
}
func (t *Type) Descriptor() *x.Type {
	if t == nil {
		return nil
	}
	return t.descriptor
}
func (t *Type) NamedDescriptor() *x.Type {
	if t == nil || t.descriptor == nil {
		return nil
	}
	if t.descriptor.Type == nil {
		return t.descriptor
	}
	base := t.descriptor.Type
	for base.Name() == "" && (base.Kind() == reflect.Pointer || base.Kind() == reflect.Slice || base.Kind() == reflect.Array || base.Kind() == reflect.Chan) {
		base = base.Elem()
	}
	return &x.Type{Type: base, Name: base.Name(), PkgPath: base.PkgPath()}
}
func (t *Type) BaseDescriptor() *x.Type {
	if t == nil || t.descriptor == nil {
		return nil
	}
	if t.descriptor.Type == nil {
		return t.descriptor
	}
	base := t.descriptor.Type
	for base.Kind() == reflect.Pointer || base.Kind() == reflect.Slice || base.Kind() == reflect.Array || base.Kind() == reflect.Chan {
		base = base.Elem()
	}
	return &x.Type{Type: base, Name: base.Name(), PkgPath: base.PkgPath()}
}
func (f Field) StructField() reflect.StructField {
	return reflect.StructField{Name: f.Name, Type: f.ReflectedType, Tag: f.Tag, Index: append([]int(nil), f.Index...), Anonymous: f.Anonymous, PkgPath: f.PkgPath}
}
func (f Field) BuiltinType() (string, bool) {
	if f.ReflectedType != nil {
		base := f.ReflectedType
		for base.Kind() == reflect.Pointer || base.Kind() == reflect.Slice || base.Kind() == reflect.Array {
			base = base.Elem()
		}
		if base.PkgPath() != "" {
			return "", false
		}
		expression, err := (Resolver{}).Expression(f.ReflectedType)
		if err != nil {
			return "", false
		}
		ref, err := (Resolver{}).Reference(expression)
		if err != nil || !builtin(ref.BaseName) {
			return "", false
		}
		return expression, true
	}
	ref, err := (Resolver{}).Reference(f.TypeExpr)
	if err != nil || ref.Qualifier != "" || !builtin(ref.BaseName) {
		return "", false
	}
	return f.TypeExpr, true
}

func (t *Type) Fields() ([]Field, error) { return t.fields(map[string]bool{}) }
func (t *Type) fields(visiting map[string]bool) ([]Field, error) {
	if t == nil || t.descriptor == nil {
		return nil, nil
	}
	if typeOf := t.descriptor.Type; typeOf != nil {
		for typeOf.Kind() == reflect.Pointer || typeOf.Kind() == reflect.Slice || typeOf.Kind() == reflect.Array {
			typeOf = typeOf.Elem()
		}
		if typeOf.Kind() != reflect.Struct {
			return nil, fmt.Errorf("type %s is not a struct", typeOf)
		}
		fields := reflect.VisibleFields(typeOf)
		result := make([]Field, 0, len(fields))
		for _, field := range fields {
			result = append(result, Field{Name: field.Name, ReflectedType: field.Type, Tag: field.Tag, Index: append([]int(nil), field.Index...), Anonymous: field.Anonymous, Exported: field.IsExported(), PkgPath: field.PkgPath})
		}
		return result, nil
	}
	declaration := t.descriptor.SynteticType
	if declaration == nil || declaration.TypeSpec == nil {
		return nil, fmt.Errorf("type %s has no structural descriptor", t.descriptor.Name)
	}
	key := t.descriptor.PkgPath + "." + t.descriptor.Name
	if visiting[key] {
		return nil, fmt.Errorf("cyclic synthetic type %s", key)
	}
	visiting[key] = true
	defer delete(visiting, key)
	expression := declaration.TypeSpec.Type
	for {
		switch value := expression.(type) {
		case *ast.StarExpr:
			expression = value.X
		case *ast.ArrayType:
			expression = value.Elt
		case *ast.ParenExpr:
			expression = value.X
		default:
			goto structure
		}
	}
structure:
	structure, ok := expression.(*ast.StructType)
	if !ok {
		resolved, err := t.resolve(rendered(expression))
		if err != nil {
			return nil, err
		}
		return resolved.fields(visiting)
	}
	if structure.Fields == nil {
		return nil, nil
	}
	var result []Field
	index := 0
	for _, source := range structure.Fields.List {
		typeExpr := rendered(source.Type)
		tag := ""
		var err error
		if source.Tag != nil {
			tag, err = strconv.Unquote(source.Tag.Value)
			if err != nil {
				return nil, err
			}
		}
		names := source.Names
		anonymous := len(names) == 0
		if anonymous {
			ref, err := (Resolver{}).Reference(typeExpr)
			if err != nil {
				return nil, err
			}
			names = []*ast.Ident{ast.NewIdent(ref.BaseName)}
		}
		for _, name := range names {
			field := Field{Name: name.Name, TypeExpr: typeExpr, Tag: reflect.StructTag(tag), Index: []int{index}, Anonymous: anonymous, Exported: token.IsExported(name.Name)}
			if !field.Exported {
				field.PkgPath = t.descriptor.PkgPath
			}
			result = append(result, field)
			if anonymous {
				embedded, err := t.resolve(typeExpr)
				if err != nil {
					return nil, err
				}
				fields, err := embedded.fields(visiting)
				if err != nil {
					return nil, err
				}
				for _, child := range fields {
					child.Index = append([]int{index}, child.Index...)
					result = append(result, child)
				}
			}
			index++
		}
	}
	// Match Go promotion: shallower names win; equal-depth competing fields
	// are ambiguous and are not visible.
	depth := map[string]int{}
	counts := map[string]int{}
	for _, field := range result {
		size := len(field.Index)
		previous, found := depth[field.Name]
		if !found || size < previous {
			depth[field.Name] = size
			counts[field.Name] = 1
		} else if size == previous {
			counts[field.Name]++
		}
	}
	visible := make([]Field, 0, len(result))
	for _, field := range result {
		if len(field.Index) == depth[field.Name] && counts[field.Name] == 1 {
			visible = append(visible, field)
		}
	}
	return visible, nil
}

func (t *Type) resolve(expression string) (*Type, error) {
	if t == nil || t.descriptor == nil {
		return nil, fmt.Errorf("type %q requires structural lookup", expression)
	}
	parsed, err := (Resolver{}).parse(expression)
	if err != nil {
		return nil, err
	}
	for {
		switch value := parsed.(type) {
		case *ast.StarExpr:
			parsed = value.X
		case *ast.ArrayType:
			parsed = value.Elt
		case *ast.ParenExpr:
			parsed = value.X
		default:
			goto unwrapped
		}
	}
unwrapped:
	if _, ok := parsed.(*ast.StructType); ok {
		declaration := &model.Type{Name: t.descriptor.Name + "#" + expression, PkgPath: t.descriptor.PkgPath, TypeSpec: &ast.TypeSpec{Type: parsed}}
		if t.descriptor.SynteticType != nil {
			declaration.Imports = t.descriptor.SynteticType.Imports
		}
		return New(&x.Type{Name: declaration.Name, PkgPath: declaration.PkgPath, SynteticType: declaration}, t.lookup), nil
	}
	if t.lookup == nil {
		return nil, fmt.Errorf("type %q requires structural lookup", expression)
	}
	resolver := Resolver{Package: t.descriptor.PkgPath, Lookup: t.lookup, Imports: map[string]string{}}
	if declaration := t.descriptor.SynteticType; declaration != nil {
		for alias, imported := range declaration.Imports {
			if imported != nil {
				resolver.Imports[alias] = imported.Path
			}
		}
	}
	resolved, err := resolver.Resolve(expression)
	if err != nil {
		return nil, err
	}
	if resolved.Descriptor == nil {
		return nil, fmt.Errorf("type %q was not resolved", expression)
	}
	return New(resolved.Descriptor, t.lookup), nil
}

func (t *Type) FieldsAt(fieldPath string) ([]Field, error) {
	if strings.TrimSpace(fieldPath) == "" {
		return t.Fields()
	}
	current := t
	for _, name := range strings.FieldsFunc(fieldPath, func(r rune) bool { return r == '.' || r == '/' }) {
		fields, err := current.Fields()
		if err != nil {
			return nil, err
		}
		var found *Field
		for i := range fields {
			if fields[i].Name == name {
				found = &fields[i]
				break
			}
		}
		if found == nil {
			return nil, fmt.Errorf("field %q was not found", name)
		}
		if found.ReflectedType != nil {
			current = Linked(found.ReflectedType)
		} else {
			current, err = current.resolve(found.TypeExpr)
			if err != nil {
				return nil, err
			}
		}
	}
	return current.Fields()
}

func (t *Type) StructField(fieldPath string) (reflect.StructField, error) {
	if t == nil || t.descriptor == nil || t.descriptor.Type == nil {
		return reflect.StructField{}, fmt.Errorf("linked struct type is required")
	}
	current := t.descriptor.Type
	var indexes []int
	var result reflect.StructField
	for _, name := range strings.Split(fieldPath, ".") {
		current = (Runtime{}).Indirect(current)
		if current == nil || current.Kind() != reflect.Struct {
			return reflect.StructField{}, fmt.Errorf("field path %q does not traverse a struct", fieldPath)
		}
		field, ok := current.FieldByName(name)
		if !ok {
			return reflect.StructField{}, fmt.Errorf("field %q was not found on %s", name, current)
		}
		indexes = append(indexes, field.Index...)
		result = field
		current = field.Type
	}
	result.Index = indexes
	return result, nil
}
func (t *Type) FirstStructField(names ...string) (reflect.StructField, bool) {
	for _, name := range names {
		field, err := t.StructField(name)
		if err == nil {
			return field, true
		}
	}
	return reflect.StructField{}, false
}

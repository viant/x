package shape

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"sort"
)

type methodNode struct {
	typ     *Type
	pointer bool
	trail   map[string]bool
}
type methodCandidate struct {
	method    Method
	available bool
}
type methodSet struct{}

// resolve follows Go's breadth-first selector search. Field names and unavailable
// pointer methods also shadow deeper methods. Distinct equal-depth paths remain
// ambiguous even if they arrive at the same embedded type/signature.
func (s *methodSet) resolve(root *Type, pointer bool) ([]Method, error) {
	queue := []methodNode{{typ: root, pointer: pointer, trail: map[string]bool{}}}
	seenNames := map[string]bool{}
	result := []Method{}
	for len(queue) > 0 {
		if err := s.checkOpaqueCompetition(queue, seenNames); err != nil {
			return nil, err
		}
		next := []methodNode{}
		candidates := map[string][]methodCandidate{}
		for _, node := range queue {
			identity := node.typ.descriptor.PkgPath + "." + node.typ.descriptor.Name
			if node.trail[identity] {
				continue
			}
			trail := map[string]bool{}
			for key, value := range node.trail {
				trail[key] = value
			}
			trail[identity] = true
			available, err := node.typ.declaredMethods(node.pointer)
			if err != nil {
				return nil, err
			}
			all, err := node.typ.declaredMethods(true)
			if err != nil {
				return nil, err
			}
			availableNames := map[string]bool{}
			for _, method := range available {
				availableNames[method.Name] = true
			}
			for _, method := range all {
				candidates[method.Name] = append(candidates[method.Name], methodCandidate{method: method, available: availableNames[method.Name]})
			}
			fields, err := node.typ.methodFields()
			if err != nil {
				return nil, err
			}
			for _, field := range fields {
				if token.IsExported(field.Name) {
					candidates[field.Name] = append(candidates[field.Name], methodCandidate{})
				}
				if !field.Anonymous {
					continue
				}
				var child *Type
				var childPointer bool
				if field.ReflectedType != nil {
					childPointer = field.ReflectedType.Kind() == reflect.Pointer
					child = Linked(field.ReflectedType)
				} else {
					reference, err := (Resolver{}).Reference(field.TypeExpr)
					if err != nil {
						return nil, err
					}
					childPointer = len(reference.Wrappers) > 0 && reference.Wrappers[0].Kind == WrapperPointer
					child, err = node.typ.resolve(field.TypeExpr)
					if err != nil {
						return nil, err
					}
				}
				next = append(next, methodNode{typ: child.promotionAuthority(), pointer: node.pointer || childPointer, trail: trail})
			}
		}
		for name, items := range candidates {
			if seenNames[name] {
				continue
			}
			seenNames[name] = true
			if len(items) == 1 && items[0].available {
				result = append(result, items[0].method)
			}
		}
		queue = next
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (t *Type) methodFields() ([]Field, error) {
	if t.descriptor.Type != nil {
		base := (Runtime{}).Indirect(t.descriptor.Type)
		if base.Kind() != reflect.Struct {
			return nil, nil
		}
		result := []Field{}
		for _, field := range reflect.VisibleFields(base) {
			// Its final Go method/field sets are already resolved. Expansion here
			// would count reflected promoted methods twice.
			result = append(result, Field{Name: field.Name, ReflectedType: field.Type, Exported: field.IsExported()})
		}
		return result, nil
	}
	declaration := t.descriptor.SynteticType
	if declaration == nil || declaration.TypeSpec == nil {
		return nil, nil
	}
	structure, ok := declaration.TypeSpec.Type.(*ast.StructType)
	if !ok {
		// Named scalar/container types may declare methods but have no promoted
		// struct fields. A defined type with named struct underlying authority
		// retains that underlying struct's embedded fields, not its methods.
		if reference, err := (Resolver{}).Reference(rendered(declaration.TypeSpec.Type)); err == nil && len(reference.Wrappers) == 0 && !builtin(reference.BaseName) {
			underlying, err := t.resolve(rendered(declaration.TypeSpec.Type))
			if err != nil {
				return nil, err
			}
			if underlying.descriptor.Key() == t.descriptor.Key() {
				return nil, fmt.Errorf("cyclic underlying method receiver %s", t.descriptor.Key())
			}
			return underlying.methodFields()
		}
		return nil, nil
	}
	if structure.Fields == nil {
		return nil, nil
	}
	result := []Field{}
	for _, source := range structure.Fields.List {
		if len(source.Names) > 0 {
			for _, name := range source.Names {
				result = append(result, Field{Name: name.Name, TypeExpr: rendered(source.Type)})
			}
			continue
		}
		expression := rendered(source.Type)
		reference, err := (Resolver{}).Reference(expression)
		if err != nil {
			return nil, err
		}
		result = append(result, Field{Name: reference.BaseName, TypeExpr: expression, Anonymous: true})
	}
	return result, nil
}

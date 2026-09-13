package shape

import (
	"fmt"
	"go/token"
	"reflect"
)

type selectorNames struct{ methods, fields map[string]bool }

// Prefer declaring source metadata over a reflected, already-promoted set.
// Descriptors synthesized from reflection identify that origin explicitly.
func (t *Type) promotionAuthority() *Type {
	if t == nil || t.descriptor == nil || t.descriptor.Type == nil {
		return t
	}
	source := t.descriptor.SynteticType
	if source == nil || source.TypeSpec == nil || source.ReflectType != nil || source.LinkedinType != nil {
		return t
	}
	descriptor := *t.descriptor
	descriptor.Type = nil
	return New(&descriptor, t.lookup)
}

// Reflection supplies exact final method sets, but not the declaring depth of
// promoted methods. Such a branch is safe when no outside selector competes;
// competing paths require source descriptors rather than a guessed depth.
func (s *methodSet) checkOpaqueCompetition(nodes []methodNode, shadowed map[string]bool) error {
	if len(nodes) < 2 {
		return nil
	}
	potential := map[int]selectorNames{}
	for index, node := range nodes {
		if !node.typ.opaquePromotion() {
			continue
		}
		left, err := s.selectorNames(node, map[string]bool{})
		if err != nil {
			return err
		}
		for other, candidate := range nodes {
			if index == other {
				continue
			}
			right, ok := potential[other]
			if !ok {
				right, err = s.selectorNames(candidate, map[string]bool{})
				if err != nil {
					return err
				}
				potential[other] = right
			}
			for name := range left.methods {
				if !shadowed[name] && (right.methods[name] || right.fields[name]) {
					return fmt.Errorf("promoted selector %s competes with embedded linked type %s; source method authority is required to determine declaration depth", name, node.typ.descriptor.Key())
				}
			}
			for name := range right.methods {
				if !shadowed[name] && left.fields[name] {
					return fmt.Errorf("promoted selector %s competes with a field in embedded linked type %s; source method authority is required to determine declaration depth", name, node.typ.descriptor.Key())
				}
			}
		}
	}
	return nil
}

func (t *Type) opaquePromotion() bool {
	if t == nil || t.descriptor == nil || t.descriptor.Type == nil {
		return false
	}
	base := (Runtime{}).Indirect(t.descriptor.Type)
	if base.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < base.NumField(); i++ {
		if base.Field(i).Anonymous {
			return true
		}
	}
	return false
}

func (s *methodSet) selectorNames(node methodNode, visited map[string]bool) (selectorNames, error) {
	result := selectorNames{methods: map[string]bool{}, fields: map[string]bool{}}
	identity := node.typ.descriptor.PkgPath + "." + node.typ.descriptor.Name
	if visited[identity] {
		return result, nil
	}
	visited[identity] = true
	methods, err := node.typ.declaredMethods(true)
	if err != nil {
		return result, err
	}
	for _, method := range methods {
		result.methods[method.Name] = true
	}
	fields, err := node.typ.methodFields()
	if err != nil {
		return result, err
	}
	if typ := node.typ.descriptor.Type; typ != nil {
		base := (Runtime{}).Indirect(typ)
		if base.Kind() == reflect.Struct {
			fields = nil
			for i := 0; i < base.NumField(); i++ {
				field := base.Field(i)
				fields = append(fields, Field{Name: field.Name, Anonymous: field.Anonymous, ReflectedType: field.Type})
			}
		}
	}
	for _, field := range fields {
		if token.IsExported(field.Name) {
			result.fields[field.Name] = true
		}
		if !field.Anonymous {
			continue
		}
		var child *Type
		if field.ReflectedType != nil {
			child = Linked(field.ReflectedType)
		} else {
			child, err = node.typ.resolve(field.TypeExpr)
			if err != nil {
				return result, err
			}
		}
		nested, err := s.selectorNames(methodNode{typ: child, pointer: true}, visited)
		if err != nil {
			return result, err
		}
		for name := range nested.methods {
			result.methods[name] = true
		}
		for name := range nested.fields {
			result.fields[name] = true
		}
	}
	return result, nil
}

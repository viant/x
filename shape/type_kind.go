package shape

import (
	"fmt"
	"go/ast"
	"reflect"
)

// IsStruct reports the actual named value's structural kind. Unlike Fields it
// does not unwrap collections into their element rows.
func (t *Type) IsStruct() (bool, error) { return t.isStruct(map[string]bool{}) }
func (t *Type) isStruct(visited map[string]bool) (bool, error) {
	if t == nil || t.descriptor == nil {
		return false, fmt.Errorf("type authority is required")
	}
	if typ := t.descriptor.Type; typ != nil {
		return (Runtime{}).Indirect(typ).Kind() == reflect.Struct, nil
	}
	identity := t.descriptor.PkgPath + "." + t.descriptor.Name
	if visited[identity] {
		return false, fmt.Errorf("cyclic type authority %s", identity)
	}
	visited[identity] = true
	source := t.descriptor.SynteticType
	if source == nil || source.TypeSpec == nil {
		return false, fmt.Errorf("type %s has no structural authority", identity)
	}
	switch expression := source.TypeSpec.Type.(type) {
	case *ast.StructType:
		return true, nil
	case *ast.ParenExpr:
		copy := *t.descriptor
		synthetic := *source
		spec := *source.TypeSpec
		spec.Type = expression.X
		synthetic.TypeSpec = &spec
		copy.SynteticType = &synthetic
		delete(visited, identity)
		return New(&copy, t.lookup).isStruct(visited)
	case *ast.Ident:
		if builtin(expression.Name) {
			return false, nil
		}
	case *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
	default:
		return false, nil
	}
	underlying, err := t.resolve(rendered(source.TypeSpec.Type))
	if err != nil {
		return false, err
	}
	return underlying.isStruct(visited)
}

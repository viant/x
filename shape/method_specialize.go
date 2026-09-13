package shape

import (
	"fmt"
	"go/ast"

	"github.com/viant/x"
	model "github.com/viant/x/syntetic/model"
)

// specializeMethods updates only the detached descriptor created by Resolve.
// Method receiver parameter names may differ from type declaration names.
func (r Resolver) specializeMethods(descriptor *x.Type, names, arguments []string) error {
	synthetic := descriptor.SynteticType
	for _, pair := range []struct {
		models       []model.Method
		declarations *[]*ast.FuncDecl
	}{{synthetic.Methods.Value, &synthetic.MethodsAST}, {synthetic.Methods.Pointer, &synthetic.PtrMethodsAST}} {
		known := map[string]bool{}
		for _, decl := range *pair.declarations {
			if decl != nil {
				known[decl.Name.Name] = true
			}
		}
		for _, method := range pair.models {
			if !known[method.Name] {
				aliases := map[string]string{}
				for alias, location := range r.methodScope(synthetic, method.Name).Imports {
					aliases[location] = alias
				}
				*pair.declarations = append(*pair.declarations, &ast.FuncDecl{Name: ast.NewIdent(method.Name), Type: method.Type.TypeAST(synthetic.PkgPath, aliases)})
			}
		}
		for _, decl := range *pair.declarations {
			if decl == nil {
				continue
			}
			parameterNames := names
			if decl.Recv != nil && len(decl.Recv.List) == 1 {
				reference, err := (Resolver{}).Reference(rendered(decl.Recv.List[0].Type))
				if err != nil {
					return err
				}
				if len(reference.Arguments) > 0 {
					if len(reference.Arguments) != len(arguments) {
						return fmt.Errorf("method %s receiver generic argument count differs from type", decl.Name.Name)
					}
					parameterNames = reference.Arguments
				}
			}
			substitute := Resolver{Rewriter: func(name string) (string, error) {
				for i, param := range parameterNames {
					if name == param {
						return arguments[i], nil
					}
				}
				return name, nil
			}}
			expression, err := substitute.rewrite(decl.Type)
			if err != nil {
				return fmt.Errorf("specialize method %s: %w", decl.Name.Name, err)
			}
			decl.Type = expression.(*ast.FuncType)
		}
	}
	// AST signatures are now the specialized authority; the original model
	// declarations remain intact on the caller's unmodified base descriptor.
	synthetic.Methods = model.MethodSet{}
	return nil
}

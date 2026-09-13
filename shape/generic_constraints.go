package shape

import (
	"fmt"
	"github.com/viant/x"
	"go/ast"
	"go/types"
)

// validateGenericConstraints delegates type-set satisfaction to go/types using
// supplied descriptors only. It never loads modules or executes application code.
func (r Resolver) validateGenericConstraints(descriptor *x.Type, parameters *ast.FieldList, arguments []string) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("generic constraint type authority: %v", recovered)
		}
	}()
	context := r
	context.Package = descriptor.PkgPath
	context.Imports = map[string]string{}
	for alias, item := range descriptor.SynteticType.Imports {
		if item != nil {
			context.Imports[alias] = item.Path
		}
	}
	builder := newGoTypeAuthority()
	bindings := map[string]types.Type{}
	index := 0
	for _, field := range parameters.List {
		for _, parameter := range field.Names {
			if !emptyConstraint(field.Type) {
				value, err := builder.expression(r, arguments[index], nil)
				if err != nil {
					return fmt.Errorf("type argument %s: %w", arguments[index], err)
				}
				bindings[parameter.Name] = value
			}
			index++
		}
	}
	index = 0
	for _, field := range parameters.List {
		for _, parameter := range field.Names {
			if emptyConstraint(field.Type) {
				index++
				continue
			}
			argumentIndex := 0
			for _, candidate := range parameters.List {
				for _, name := range candidate.Names {
					if bindings[name.Name] == nil {
						value, err := builder.expression(r, arguments[argumentIndex], nil)
						if err != nil {
							return err
						}
						bindings[name.Name] = value
					}
					argumentIndex++
				}
			}
			constraint, err := builder.syntax(context, field.Type, bindings)
			if err != nil {
				return fmt.Errorf("constraint for %s: %w", parameter.Name, err)
			}
			contract, ok := constraint.Underlying().(*types.Interface)
			if !ok {
				contract = types.NewInterfaceType(nil, []types.Type{constraint})
			}
			contract.Complete()
			if contract.NumMethods() > 0 {
				if err := r.constraintMethodAuthority(arguments[index]); err != nil {
					return err
				}
			}
			if !types.Satisfies(bindings[parameter.Name], contract) {
				return fmt.Errorf("type argument %s does not satisfy %s constraint for %s", arguments[index], rendered(field.Type), parameter.Name)
			}
			index++
		}
	}
	return nil
}

// A reflected method set has no declaring-depth metadata. The native selector
// owner rejects only genuinely competing opaque promotion paths before their
// flattened methods can influence go/types constraint satisfaction.
func (r Resolver) constraintMethodAuthority(argument string) error {
	reference, err := r.Reference(argument)
	if err != nil {
		return err
	}
	if builtin(reference.BaseName) || reference.BaseName == "" {
		return nil
	}
	if len(reference.Wrappers) > 1 || len(reference.Wrappers) == 1 && reference.Wrappers[0].Kind != WrapperPointer {
		return nil
	}
	resolved, err := r.Resolve(argument)
	if err != nil {
		return err
	}
	if resolved == nil || resolved.Descriptor == nil {
		return fmt.Errorf("method constraint argument %s has no supplied authority", argument)
	}
	_, err = New(resolved.Descriptor, r.Lookup).Methods(len(reference.Wrappers) == 1)
	return err
}
func emptyConstraint(expression ast.Expr) bool {
	if identifier, ok := expression.(*ast.Ident); ok && identifier.Name == "any" {
		return true
	}
	if constraint, ok := expression.(*ast.InterfaceType); ok && (constraint.Methods == nil || len(constraint.Methods.List) == 0) {
		return true
	}
	return false
}

package shape

import (
	"fmt"
	"strings"
)

// fieldAuthority is a detached source-context snapshot. It deliberately holds
// no descriptor, lookup function, or externally accessible import map.
type fieldAuthority struct {
	packagePath string
	imports     map[string]string
}

func (t *Type) fieldAuthority() *fieldAuthority {
	result := &fieldAuthority{packagePath: t.descriptor.PkgPath, imports: map[string]string{}}
	if declaration := t.descriptor.SynteticType; declaration != nil {
		if declaration.PkgPath != "" {
			result.packagePath = declaration.PkgPath
		}
		for alias, imported := range declaration.Imports {
			if imported != nil {
				result.imports[alias] = imported.Path
			}
		}
	}
	return result
}

// CanonicalType returns the complete field type expression under its actual
// declaring package/import authority. Promoted synthetic fields retain that
// authority, detached from later descriptor mutations. TypeExpr is unchanged.
func (f Field) CanonicalType() (string, error) {
	if f.ReflectedType != nil {
		return (Resolver{Qualifier: func(pkg, _ string) string { return pkg }}).Expression(f.ReflectedType)
	}
	if f.authority == nil {
		return "", fmt.Errorf("field %q has no declaring type authority", f.Name)
	}
	return (Resolver{Package: f.authority.packagePath, Imports: f.authority.imports}).Canonical(f.TypeExpr)
}

// ResolveField resolves an exported path using native visible-field selection.
// Identity is the complete canonical field type. Descriptor follows Resolver's
// named-type lookup contract and may be nil for builtin/unregistered types.
func (t *Type) ResolveField(path string) (*Resolution, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("field path is required")
	}
	current := t
	names := strings.Split(path, ".")
	for index, name := range names {
		if name == "" || name != strings.TrimSpace(name) {
			return nil, fmt.Errorf("invalid field path %q", path)
		}
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
		if found == nil || !found.Exported {
			return nil, fmt.Errorf("exported field %q was not found at %q", name, path)
		}
		if index == len(names)-1 {
			identity, err := found.CanonicalType()
			if err != nil {
				return nil, err
			}
			if found.ReflectedType != nil {
				return &Resolution{Identity: identity, Descriptor: Linked(found.ReflectedType).NamedDescriptor()}, nil
			}
			// CanonicalType has validated the full expression. Unnamed maps,
			// functions and inline structs do not have a named lookup target.
			if _, err := (Resolver{}).Reference(identity); err != nil {
				return &Resolution{Identity: identity}, nil
			}
			return (Resolver{Lookup: current.lookup}).Resolve(identity)
		}
		current, err = current.fieldShape(*found)
		if err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("field path %q was not resolved", path)
}

func (t *Type) fieldShape(field Field) (*Type, error) {
	if field.ReflectedType != nil {
		return Linked(field.ReflectedType), nil
	}
	identity, err := field.CanonicalType()
	if err != nil {
		return nil, err
	}
	return t.resolve(identity)
}

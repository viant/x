package xreflect

import (
	"fmt"
	"go/ast"
	"reflect"

	"github.com/viant/x/loader/xreflect/xtype"
	"github.com/viant/x/syntetic/model"
)

// LoadPackage constructs a model.Package from the seed reflect.Type rType and
// optional types supplied via LoadOption. The package path and name default to
// rType.PkgPath() and its last segment unless overridden via options.
//
// Notes:
//   - Only named types are emitted as top-level declarations.
//   - Types from other packages are skipped by default; enable WithAllowCrossPackage
//     to suppress errors about such types (they are still skipped to keep a
//     coherent package output).
func LoadPackage(rType reflect.Type, opts ...LoadOption) (*model.Package, error) {
	c := &cfg{
		namePolicy: func(rt reflect.Type) (string, bool) { return rt.Name(), false },
	}
	if rType != nil {
		c.types = append(c.types, rType)
		if c.pkgPath == "" {
			c.pkgPath = rType.PkgPath()
		}
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	if c.pkgPath == "" {
		return nil, fmt.Errorf("xreflect: package path is required (provide a non-dynamic rType or use WithPackagePath)")
	}
	if c.pkgName == "" {
		c.pkgName = lastSegment(c.pkgPath)
	}
	if len(c.types) == 0 {
		return nil, fmt.Errorf("xreflect: at least one type is required (seed rType or WithTypes/WithValues)")
	}

	p := &model.Package{Name: c.pkgName, PkgPath: c.pkgPath}
	imports := xtype.NewImportSet(c.pkgPath)
	seen := map[string]struct{}{}

	for _, rt := range c.types {
		if rt == nil {
			continue
		}
		// Enforce same package: skip others; optionally suppress error.
		if rt.PkgPath() != c.pkgPath {
			if c.allowCross {
				continue
			}
			return nil, fmt.Errorf("xreflect: type %s from %s does not match package %s (use WithAllowCrossPackage to ignore)", rt.String(), rt.PkgPath(), c.pkgPath)
		}
		declName, _ := c.namePolicy(rt)
		if declName == "" {
			declName = rt.Name()
		}
		if declName == "" {
			continue
		}
		key := rt.PkgPath() + "." + declName
		if _, dup := seen[key]; dup {
			continue
		}

		node, err := xtype.FromReflect(rt)
		if err != nil {
			if c.onUnnamed != nil && err == xtype.ErrUnnamedRecursion {
				if c.onUnnamed(rt) == nil {
					continue
				}
			}
			return nil, err
		}
		if node == nil {
			continue
		}
		expr, err := xtype.ToAST(node, imports)
		if err != nil {
			return nil, err
		}
		spec := &ast.TypeSpec{Name: ast.NewIdent(declName), Type: expr}
		t := &model.Type{
			Name:         declName,
			PkgPath:      c.pkgPath,
			ReflectType:  rt,
			LinkedinType: rt,
			TypeSpec:     spec,
			Imports:      translateImports(imports),
		}
		p.AddType(t)
		seen[key] = struct{}{}
	}
	return p, nil
}

// BuildType constructs a single model.Type from the provided reflect.Type.
// Options such as WithPackagePath/WithPackageName/WithNamePolicy and
// WithOnUnnamedRecursion are honoured. Cross-package checks are not applied
// for single-type conversion.
func BuildType(rt reflect.Type, opts ...LoadOption) (*model.Type, error) {
	if rt == nil {
		return nil, fmt.Errorf("xreflect: reflect.Type is required")
	}
	c := &cfg{
		namePolicy: func(rt reflect.Type) (string, bool) { return rt.Name(), false },
	}
	c.types = append(c.types, rt)
	if c.pkgPath == "" {
		c.pkgPath = rt.PkgPath()
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	if c.pkgPath == "" {
		return nil, fmt.Errorf("xreflect: package path is required (use WithPackagePath)")
	}
	name, _ := c.namePolicy(rt)
	if name == "" {
		name = rt.Name()
	}
	if name == "" {
		return nil, fmt.Errorf("xreflect: type has no name")
	}
	imps := xtype.NewImportSet(c.pkgPath)
	node, err := xtype.FromReflect(rt)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, nil
	}
	expr, err := xtype.ToAST(node, imps)
	if err != nil {
		return nil, err
	}
	spec := &ast.TypeSpec{Name: ast.NewIdent(name), Type: expr}
	t := &model.Type{
		Name:         name,
		PkgPath:      c.pkgPath,
		ReflectType:  rt,
		LinkedinType: rt,
		TypeSpec:     spec,
		Imports:      translateImports(imps),
	}
	return t, nil
}

func translateImports(imps *xtype.ImportSet) map[string]*model.ImportRef {
	if imps == nil {
		return nil
	}
	entries := imps.Entries()
	if len(entries) == 0 {
		return nil
	}
	out := make(map[string]*model.ImportRef, len(entries))
	for _, e := range entries {
		out[e.Alias] = &model.ImportRef{Path: e.Path, Alias: e.Alias}
	}
	return out
}

func lastSegment(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}

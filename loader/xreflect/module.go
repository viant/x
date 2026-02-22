package xreflect

import (
	"fmt"
	"reflect"

	"github.com/viant/x/syntetic/model"
)

// ModuleOption configures LoadModuleByNames behaviour.
type ModuleOption func(*mcfg)

type mcfg struct {
	allowCross bool
	onUnnamed  func(reflect.Type) error
	namePolicy func(reflect.Type) (string, bool)
	// assembly inputs
	names     []string
	provider  func(pkgPath string) []reflect.Type
	pkgTypes  map[string][]reflect.Type // pkgPath -> types
	pkgNames  map[string]string         // pkgPath -> pkgName override
	seedTypes []reflect.Type
}

// WithModuleAllowCrossPackage suppresses errors about cross-package types.
func WithModuleAllowCrossPackage() ModuleOption { return func(c *mcfg) { c.allowCross = true } }

// WithModuleOnUnnamedRecursion installs a callback for unnamed recursive composites.
func WithModuleOnUnnamedRecursion(fn func(reflect.Type) error) ModuleOption {
	return func(c *mcfg) { c.onUnnamed = fn }
}

// WithModuleNamePolicy customises declared names for types.
func WithModuleNamePolicy(fn func(reflect.Type) (string, bool)) ModuleOption {
	return func(c *mcfg) { c.namePolicy = fn }
}

// WithModuleNamesAndProvider sets a list of package import paths and a provider
// returning reflect types for each package.
func WithModuleNamesAndProvider(names []string, provider func(pkgPath string) []reflect.Type) ModuleOption {
	return func(c *mcfg) { c.names = append(c.names, names...); c.provider = provider }
}

// WithModulePackageTypes adds types for the specified package import path.
func WithModulePackageTypes(pkgPath string, types ...reflect.Type) ModuleOption {
	return func(c *mcfg) {
		if c.pkgTypes == nil {
			c.pkgTypes = map[string][]reflect.Type{}
		}
		c.pkgTypes[pkgPath] = append(c.pkgTypes[pkgPath], types...)
	}
}

// WithModulePackageName overrides the package name for the given import path.
func WithModulePackageName(pkgPath, name string) ModuleOption {
	return func(c *mcfg) {
		if c.pkgNames == nil {
			c.pkgNames = map[string]string{}
		}
		c.pkgNames[pkgPath] = name
	}
}

// WithModuleSeedTypes groups the provided types by their reflect package and
// includes them in the resulting module.
func WithModuleSeedTypes(types ...reflect.Type) ModuleOption {
	return func(c *mcfg) { c.seedTypes = append(c.seedTypes, types...) }
}

// LoadModule constructs a model.Module from sources specified via ModuleOption.
// Sources can include named package lists with a provider, explicit package→types
// mappings, and raw seed types that will be grouped by their reflect package.
func LoadModule(opts ...ModuleOption) (*model.Module, error) {
	c := &mcfg{}
	for _, o := range opts {
		if o != nil {
			o(c)
		}
	}
	// Assemble package map
	pkgMap := map[string][]reflect.Type{}
	// From explicit package types
	for p, list := range c.pkgTypes {
		if len(list) > 0 {
			pkgMap[p] = append(pkgMap[p], list...)
		}
	}
	// From names + provider
	if len(c.names) > 0 {
		if c.provider == nil {
			return nil, fmt.Errorf("xreflect: provider is required when names are supplied")
		}
		for _, p := range c.names {
			if p == "" {
				continue
			}
			if ts := c.provider(p); len(ts) > 0 {
				pkgMap[p] = append(pkgMap[p], ts...)
			}
		}
	}
	// From seed types grouped by PkgPath
	for _, rt := range c.seedTypes {
		if rt == nil || rt.PkgPath() == "" {
			continue
		}
		p := rt.PkgPath()
		pkgMap[p] = append(pkgMap[p], rt)
	}
	// Build module
	mod := &model.Module{}
	for pkgPath, types := range pkgMap {
		if len(types) == 0 {
			continue
		}
		name := c.pkgNames[pkgPath]
		if name == "" {
			name = lastSegment(pkgPath)
		}
		lopts := []LoadOption{WithPackagePath(pkgPath), WithPackageName(name), WithTypes(types...)}
		if c.allowCross {
			lopts = append(lopts, WithAllowCrossPackage())
		}
		if c.onUnnamed != nil {
			lopts = append(lopts, WithOnUnnamedRecursion(c.onUnnamed))
		}
		if c.namePolicy != nil {
			lopts = append(lopts, WithNamePolicy(c.namePolicy))
		}
		p, err := LoadPackage(nil, lopts...)
		if err != nil {
			return nil, err
		}
		if p != nil {
			mod.AddPackage(p)
		}
	}
	return mod, nil
}

// LoadModuleByNames builds a model.Module given package import paths and a provider
// that returns the reflect.Type list for each package. pkgNames are import paths
// (e.g., "github.com/acme/p").
//
// Typical usage with an external registry:
//
//	names := xunsafe.PackageNames()
//	mod, _ := xreflect.LoadModuleByNames(names, xunsafe.PackageTypesFor)
func LoadModuleByNames(pkgNames []string, provider func(pkgPath string) []reflect.Type, opts ...ModuleOption) (*model.Module, error) {
	// Wrapper to the unified LoadModule; retains existing API.
	options := []ModuleOption{WithModuleNamesAndProvider(pkgNames, provider)}
	options = append(options, opts...)
	return LoadModule(options...)
}

// LoadModuleFromMap is a convenience that accepts a map of pkgPath -> []reflect.Type.
func LoadModuleFromMap(typeMap map[string][]reflect.Type, opts ...ModuleOption) (*model.Module, error) {
	options := make([]ModuleOption, 0, len(typeMap)+len(opts)+1)
	for p, ts := range typeMap {
		types := ts
		options = append(options, WithModulePackageTypes(p, types...))
	}
	options = append(options, opts...)
	return LoadModule(options...)
}

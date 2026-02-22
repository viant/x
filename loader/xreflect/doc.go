// Package xreflect provides a reflect-based loader that builds a model.Package
// or model.Module from reflect.Type values. It complements the AST-based
// loader by enabling runtime discovery via registries or aggregator variables
// (e.g., a slice of types exported by a package).
//
// Entry points
//   - LoadPackage(rType reflect.Type, opts ...LoadOption):
//     Builds a single package using a seed type (for default pkg path/name)
//     and additional types provided through options.
//   - LoadModule(opts ...ModuleOption):
//     Builds a module from multiple sources (names+provider, explicit
//     package→types, or seed types grouped by their reflect package).
//
// Typical usage
//
//	// Single package derived from a seed type, plus extras:
//	//   pkg, _ := xreflect.LoadPackage(reflect.TypeOf(MyType{}), xreflect.WithTypes(reflect.TypeOf(Other{})))
//
//	// Module from a registry that exposes package names and a provider:
//	//   mod, _ := xreflect.LoadModule(
//	//       xreflect.WithModuleNamesAndProvider(xunsafe.PackageNames(), xunsafe.PackageTypesFor),
//	//   )
//
// Options overview
//
//	LoadOption:
//	  - WithPackagePath(string), WithPackageName(string)
//	  - WithTypes(...reflect.Type), WithValues(...any), WithTypesFrom(func() []reflect.Type)
//	  - WithAllowCrossPackage() // suppress errors for cross-package types (skipped)
//	  - WithOnUnnamedRecursion(func(reflect.Type) error)
//	  - WithNamePolicy(func(reflect.Type) (declName string, ok bool))
//
//	ModuleOption:
//	  - WithModuleNamesAndProvider([]string, func(pkg string) []reflect.Type)
//	  - WithModulePackageTypes(pkgPath string, types ...reflect.Type)
//	  - WithModulePackageName(pkgPath, name string)
//	  - WithModuleSeedTypes(...reflect.Type) // grouped by reflect package
//	  - WithModuleAllowCrossPackage(), WithModuleOnUnnamedRecursion(...), WithModuleNamePolicy(...)
//
// Limitations
//   - Reflection cannot enumerate all types in a package by itself; callers
//     must provide at least one seed type and/or a registry/provider.
//   - Generic type parameters and union constraints are not exposed by reflect
//     and therefore are not populated in the model. Use the AST loader for
//     complete generic fidelity.
//   - Free functions, consts, and vars are not discoverable via reflect.
package xreflect

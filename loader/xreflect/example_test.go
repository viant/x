package xreflect_test

import (
	"fmt"
	"reflect"

	xr "github.com/viant/x/loader/xreflect"
)

// ExampleLoadPackage shows building a package from reflect types and printing
// rendered source for the default file.
func ExampleLoadPackage() {
	type Person struct{ Name string }
	type Team struct{ Members []Person }

	// Derive pkg path/name from seed type and include additional types.
	pkg, err := xr.LoadPackage(reflect.TypeOf(Person{}), xr.WithTypes(reflect.TypeOf(Team{})))
	if err != nil {
		panic(err)
	}
	gf := pkg.DefaultFile("xreflect_example")
	for _, t := range pkg.Types {
		gf.AddType(t)
	}
	src, _ := gf.Render()
	fmt.Println(len(src) > 0)
	// Output: true
}

// ExampleLoadModule demonstrates loading multiple packages by name using a
// provider that returns types for a given import path. In real life, provider
// could be xunsafe.PackageTypesFor, and names from xunsafe.PackageNames().
func ExampleLoadModule() {
	// Fake registry of package -> types for example purposes.
	type A struct{}
	type B struct{}
	type C struct{}
	registry := map[string][]reflect.Type{
		"example.org/pkg/a": {reflect.TypeOf(A{})},
		"example.org/pkg/b": {reflect.TypeOf(B{}), reflect.TypeOf(C{})},
	}
	names := []string{"example.org/pkg/a", "example.org/pkg/b"}
	mod, err := xr.LoadModuleByNames(names, func(pkg string) []reflect.Type { return registry[pkg] }, xr.WithModuleAllowCrossPackage())
	if err != nil {
		panic(err)
	}
	fmt.Println(mod.HasPackage("example.org/pkg/a") && mod.HasPackage("example.org/pkg/b"))
	// Output: true
}

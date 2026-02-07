package main

import (
	"fmt"
	"log"
	"reflect"

	x "github.com/viant/x"
	"github.com/viant/x/loader/xreflect"
	"github.com/viant/x/syntetic"
)

// This example demonstrates the flow:
//   reflect.Type -> x.Registry -> syntetic.FromRegistryFile -> Go source
// It registers a couple of types in a runtime registry, converts them to a
// model.GoFile using the syntetic bridge, and renders a compilable Go file
// as a string.

// Domain types we want to register/reflect.
type Person struct {
	ID   int
	Name string
}

type Order struct {
	Number string
	Buyer  Person
	Items  []string
}

func buildRegistry() *x.Registry {
	r := x.NewRegistry()
	// Register named, non-pointer types for cleaner names/keys and attach
	// prebuilt synthetic types to avoid reflect conversion later.
	if st, err := xreflect.BuildType(reflect.TypeOf(Person{})); err == nil {
		r.Register(x.NewType(reflect.TypeOf(Person{}), x.WithSynteticType(st)))
	} else {
		log.Fatalf("build synthetic type (Person): %v", err)
	}
	if st, err := xreflect.BuildType(reflect.TypeOf(Order{})); err == nil {
		r.Register(x.NewType(reflect.TypeOf(Order{}), x.WithSynteticType(st)))
	} else {
		log.Fatalf("build synthetic type (Order): %v", err)
	}
	return r
}

func main() {
	reg := buildRegistry()

	// Convert the registry to a single Go file.
	file, err := syntetic.FromRegistryFile(reg)
	if err != nil {
		log.Fatalf("bridge error: %v", err)
	}

	// Choose a package name for the generated file (arbitrary for this demo).
	file.PkgName = "example"

	// Optionally, you could add side-effect imports or tweak aliasing here,
	// e.g., file.AddSideEffectImport("github.com/acme/initpkg")

	src, err := file.Render()
	if err != nil {
		log.Fatalf("render error: %v", err)
	}
	fmt.Println(src)
}

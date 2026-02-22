package xreflect_test

import (
	"fmt"
	"reflect"

	xr "github.com/viant/x/builder/xreflect"
	mdl "github.com/viant/x/syntetic/model"
	trf "github.com/viant/x/syntetic/model/transform"
)

type exResolver map[string]reflect.Type

func (m exResolver) Resolve(pkgPath, name string) (reflect.Type, bool) {
	t, ok := m[pkgPath+"."+name]
	return t, ok
}

type exInterfaceResolver struct {
	t reflect.Type
}

func (r exInterfaceResolver) ResolveInterface(methods []mdl.Method) (reflect.Type, bool) {
	if len(methods) == 1 && methods[0].Name == "Read" {
		return r.t, true
	}
	return nil, false
}

// Example: tag-driven field rewrites with a resolver mapping named types.
func Example_fromTags() {
	// Build a model.Struct with tags that instruct codegen:
	// - omit field A
	// - rename field B to R, set tag json:"r", and set type to ex/p.Alias
	n := &mdl.Struct{Fields: []mdl.Field{
		{Name: "A", Tag: `x:"omit"`, Type: &mdl.Basic{Name: "int"}},
		{Name: "B", Tag: `x:"rename=R,tag=json:\"r\",type=ex/p.Alias"`, Type: &mdl.Basic{Name: "string"}},
	}}

	// Map ex/p.Alias to a concrete runtime type.
	res := exResolver{"ex/p.Alias": reflect.TypeOf(uint16(0))}
	b := xr.New(xr.WithResolver(res), xr.WithTransforms(trf.FromTags()))

	rt, _ := b.BuildNode(n)
	f := rt.Field(0)
	fmt.Println(f.Name, f.Type.String(), string(f.Tag))
	// Output: R uint16 json:"r"
}

// Example: strict interface resolution with a method-set resolver.
func Example_interfaceResolverStrict() {
	type Reader interface {
		Read([]byte) (int, error)
	}

	n := &mdl.Interface{
		Methods: []mdl.Method{
			{
				Name: "Read",
				Type: mdl.Func{
					Params: []mdl.Field{{Type: &mdl.Slice{Elem: &mdl.Basic{Name: "byte"}}}},
					Results: []mdl.Field{
						{Type: &mdl.Basic{Name: "int"}},
						{Type: &mdl.Basic{Name: "error"}},
					},
				},
			},
		},
	}

	want := reflect.TypeOf((*Reader)(nil)).Elem()
	b := xr.New(
		xr.WithInterfaceResolver(exInterfaceResolver{t: want}),
		xr.WithStrictInterfaceResolution(true),
	)
	rt, _ := b.BuildNode(n)
	fmt.Println(rt == want)
	// Output: true
}

// Example: strict union/type-set handling reports unsupported runtime shape.
func Example_strictUnionResolution() {
	n := &mdl.Union{
		Terms: []mdl.Term{
			{Type: &mdl.Basic{Name: "int"}, Approx: true},
			{Type: &mdl.Basic{Name: "string"}},
		},
	}

	b := xr.New(xr.WithStrictUnionResolution(true))
	_, err := b.BuildNode(n)
	fmt.Println(err != nil)
	// Output: true
}

// Example: alias reporter captures declaration metadata while runtime uses target type.
func Example_aliasReporter() {
	alias := &mdl.Alias{
		PkgPath: "example.com/p",
		Name:    "IDs",
		Target:  &mdl.Slice{Elem: &mdl.Basic{Name: "int"}},
	}

	var captured string
	b := xr.New(
		xr.WithAliasReporter(func(a *mdl.Alias, resolved reflect.Type) {
			captured = a.PkgPath + "." + a.Name + " -> " + resolved.String()
		}),
	)
	rt, _ := b.BuildNode(alias)
	fmt.Println(rt.String())
	fmt.Println(captured)
	// Output:
	// []int
	// example.com/p.IDs -> []int
}

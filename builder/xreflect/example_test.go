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

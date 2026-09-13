package shape_test

import (
	"context"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/viant/x"
	builder "github.com/viant/x/builder/xreflect"
	loader "github.com/viant/x/loader/ast"
	reflectloader "github.com/viant/x/loader/xreflect"
	"github.com/viant/x/shape"
	"github.com/viant/x/syntetic/model"
)

func TestLoadedMethodSignatureRoundtrip(t *testing.T) {
	for _, tc := range []struct {
		name, signature string
		want            reflect.Type
	}{
		{"grouped parameters", "(a,b int)", reflect.TypeOf(func(int, int) {})},
		{"grouped results", "() (a,b int)", reflect.TypeOf(func() (int, int) { return 0, 0 })},
		{"nested grouped callback", "(f func(a,b int)(c,d string))", reflect.TypeOf(func(func(int, int) (string, string)) {})},
		{"channel callback", "(f chan func(a,b int))", reflect.TypeOf(func(chan func(int, int)) {})},
		{"struct callback", "(f struct{F func(a,b int)})", reflect.TypeOf(func(struct{ F func(int, int) }) {})},
		{"variadic", "(values ...int)", reflect.TypeOf(func(...int) {})},
		{"variadic slices", "(prefix string, values ...[]int)", reflect.TypeOf(func(string, ...[]int) {})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pkg, err := loader.LoadPackageFS(context.Background(), fstest.MapFS{
				"go.mod": {Data: []byte("module example.com/p\n")},
				"p.go":   {Data: []byte("package p\ntype Row struct{}\nfunc(Row) Call" + tc.signature + " {}")},
			}, ".")
			if err != nil {
				t.Fatal(err)
			}
			synthetic := pkg.Types[0]
			actual, err := builder.New(builder.WithStrictNamedResolution(true)).BuildNode(&synthetic.Methods.Value[0].Type)
			if err != nil || actual != tc.want {
				t.Fatalf("runtime signature: %v, %v; want %v", actual, err, tc.want)
			}
			descriptor := &x.Type{Name: "Row", PkgPath: pkg.PkgPath, SynteticType: synthetic}
			methods, err := shape.New(descriptor, nil).Methods(false)
			if err != nil {
				t.Fatalf("AST/model conflict: %v", err)
			}
			clone, err := (x.Cloner{}).Type(descriptor)
			if err != nil {
				t.Fatal(err)
			}
			clone.SynteticType.MethodsAST = nil
			modelOnly, err := shape.New(clone, nil).Methods(false)
			if err != nil || !reflect.DeepEqual(methods, modelOnly) {
				t.Fatalf("model-only signature: %#v, %v; want %#v", modelOnly, err, methods)
			}
			reflected, err := reflectloader.ToModelNode(tc.want)
			if err != nil {
				t.Fatal(err)
			}
			clone.SynteticType.Methods.Value[0].Type = *reflected.(*model.Func)
			fromReflect, err := shape.New(clone, nil).Methods(false)
			if err != nil || !reflect.DeepEqual(methods, fromReflect) {
				t.Fatalf("reflect model signature: %#v, %v; want %#v", fromReflect, err, methods)
			}
		})
	}
}

func TestCanonicalNestedFunctionNames(t *testing.T) {
	for _, pair := range [][2]string{
		{"chan func(a,b int)", "chan func(int,int)"},
		{"struct{Callback func(a,b int)(x,y string)}", "struct{Callback func(int,int)(string,string)}"},
		{"interface{Call(a,b int)(x,y string)}", "interface{Call(int,int)(string,string)}"},
		{"interface{Call(cb chan func(value int))}", "interface{Call(chan func(int))}"},
	} {
		a, err := (shape.Resolver{}).Canonical(pair[0])
		if err != nil {
			t.Fatal(err)
		}
		b, err := (shape.Resolver{}).Canonical(pair[1])
		if err != nil {
			t.Fatal(err)
		}
		if a != b {
			t.Fatalf("equivalent types differ: %s / %s", a, b)
		}
	}
	a, _ := (shape.Resolver{}).Canonical("interface{Call(value int)}")
	b, _ := (shape.Resolver{}).Canonical("interface{Other(value int)}")
	if a == b {
		t.Fatal("interface method identity erased")
	}
}

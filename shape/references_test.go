package shape

import (
	"context"
	"github.com/viant/x"
	loader "github.com/viant/x/loader/ast"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestReferencesPreserveNamedAuthority(t *testing.T) {
	r := Resolver{Package: "example.com/app", Imports: map[string]string{"m": "example.com/model"}}
	refs, err := r.References("struct{First *m.Row; Next []Node; Index map[m.Key]Box[m.Row]; Call func(m.Row) error}")
	want := []string{"example.com/app.Box[example.com/model.Row]", "example.com/app.Node", "example.com/model.Key", "example.com/model.Row"}
	if err != nil || !reflect.DeepEqual(refs, want) {
		t.Fatalf("refs %v %v", refs, err)
	}
	pkg, err := loader.LoadPackageFS(context.Background(), fstest.MapFS{"go.mod": {Data: []byte("module example.com/app")}, "a.go": {Data: []byte("package app\nimport m \"example.com/model\"\ntype Input struct{Row *m.Row;Next *Input}")}}, ".")
	if err != nil {
		t.Fatal(err)
	}
	refs, err = New(&x.Type{Name: "Input", PkgPath: pkg.PkgPath, SynteticType: pkg.Types[0]}, nil).References()
	if err != nil || !reflect.DeepEqual(refs, []string{"example.com/app.Input", "example.com/model.Row"}) {
		t.Fatalf("descriptor refs %v %v", refs, err)
	}
}

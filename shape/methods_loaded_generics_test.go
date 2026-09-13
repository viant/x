package shape_test

import (
	"context"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/viant/x"
	loader "github.com/viant/x/loader/ast"
	"github.com/viant/x/shape"
)

func TestLoadedGenericMethodSignatures(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod": {Data: []byte("module example.com/rows\n\ngo 1.21\n")},
		"row.go": {Data: []byte(`package rows
import h "example.com/hooks"
import a "example.com/arguments"
type Row struct { ID int }
func (*Row) SyncPresence(original h.Snapshot[Row]) error { return nil }
func (Row) Transform(value h.Pair[*Row, []a.Value]) *h.Snapshot[map[string]a.Value] { return nil }
`)},
	}
	pkg, err := loader.LoadPackageFS(context.Background(), fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkg.Types) != 1 {
		t.Fatalf("types: %v", pkg.Types)
	}
	descriptor := &x.Type{Name: "Row", PkgPath: pkg.PkgPath, SynteticType: pkg.Types[0]}
	want := []shape.Method{
		{Name: "SyncPresence", Parameters: []string{"example.com/hooks.Snapshot[example.com/rows.Row]"}, Results: []string{"error"}},
		{Name: "Transform", Parameters: []string{"example.com/hooks.Pair[*example.com/rows.Row,[]example.com/arguments.Value]"}, Results: []string{"*example.com/hooks.Snapshot[map[string]example.com/arguments.Value]"}},
	}
	for _, modelOnly := range []bool{false, true} {
		copy, err := (x.Cloner{}).Type(descriptor)
		if err != nil {
			t.Fatal(err)
		}
		if modelOnly {
			copy.SynteticType.MethodsAST = nil
			copy.SynteticType.PtrMethodsAST = nil
		}
		actual, err := shape.New(copy, nil).Methods(true)
		if err != nil {
			t.Fatalf("modelOnly=%v: %v", modelOnly, err)
		}
		if !reflect.DeepEqual(want, actual) {
			t.Fatalf("modelOnly=%v: want %#v, got %#v", modelOnly, want, actual)
		}
		actual, err = shape.New(copy, nil).Methods(false)
		if err != nil || !reflect.DeepEqual(want[1:], actual) {
			t.Fatalf("value methods: %#v, %v", actual, err)
		}
	}
}

func TestLoadedMethodFileScopes(t *testing.T) {
	for _, typeFile := range []string{"0_types.go", "z_types.go"} {
		t.Run(typeFile, func(t *testing.T) {
			fsys := fstest.MapFS{
				"go.mod": {Data: []byte("module example.com/rows\n\ngo 1.21\n")},
				typeFile: {Data: []byte(`package rows
import h "example.com/fields"
type Row struct { Value h.Value }
`)},
				"a_methods.go": {Data: []byte(`package rows
import h "example.com/first"
func (*Row) First(value h.Snapshot[Row]) error { return nil }
`)},
				"b_methods.go": {Data: []byte(`package rows
import h "example.com/second"
func (Row) Second(value h.Snapshot[Row]) error { return nil }
`)},
			}
			pkg, err := loader.LoadPackageFS(context.Background(), fsys, ".")
			if err != nil {
				t.Fatal(err)
			}
			descriptor := &x.Type{Name: "Row", PkgPath: pkg.PkgPath, SynteticType: pkg.Types[0]}
			want := []shape.Method{
				{Name: "First", Parameters: []string{"example.com/first.Snapshot[example.com/rows.Row]"}, Results: []string{"error"}},
				{Name: "Second", Parameters: []string{"example.com/second.Snapshot[example.com/rows.Row]"}, Results: []string{"error"}},
			}
			for _, modelOnly := range []bool{false, true} {
				copy, err := (x.Cloner{}).Type(descriptor)
				if err != nil {
					t.Fatal(err)
				}
				if modelOnly {
					copy.SynteticType.MethodsAST = nil
					copy.SynteticType.PtrMethodsAST = nil
				}
				actual, err := shape.New(copy, nil).Methods(true)
				if err != nil || !reflect.DeepEqual(want, actual) {
					t.Fatalf("modelOnly=%v: methods=%#v, err=%v", modelOnly, actual, err)
				}
				copy.SynteticType.MethodImports["First"]["h"].Path = "example.com/changed"
				if descriptor.SynteticType.MethodImports["First"]["h"].Path != "example.com/first" {
					t.Fatal("cloned method scope aliases source")
				}
			}
			if descriptor.SynteticType.Imports["h"].Path != "example.com/fields" {
				t.Fatal("method imports replaced field scope")
			}
		})
	}
}

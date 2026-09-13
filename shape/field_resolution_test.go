package shape

import (
	"go/ast"
	"go/parser"
	"reflect"
	"testing"

	"github.com/viant/x"
	"github.com/viant/x/syntetic/model"
)

func TestFieldResolutionUsesDeclaringAuthority(t *testing.T) {
	types := map[string]*x.Type{}
	declare := func(pkg, name, source string, imports map[string]string) *x.Type {
		t.Helper()
		expression, err := parser.ParseExpr(source)
		if err != nil {
			t.Fatal(err)
		}
		declaration := &model.Type{Name: name, PkgPath: pkg, TypeSpec: &ast.TypeSpec{Name: ast.NewIdent(name), Type: expression}, Imports: map[string]*model.ImportRef{}}
		for alias, path := range imports {
			declaration.Imports[alias] = &model.ImportRef{Path: path}
		}
		result := &x.Type{Name: name, PkgPath: pkg, SynteticType: declaration}
		types[pkg+"."+name] = result
		return result
	}
	declare("example.com/first", "Value", "struct { Number int }", nil)
	declare("example.com/second", "Value", "struct { Text string }", nil)
	embedded := declare("example.com/inner", "Inner", "struct { Promoted *shared.Value; hidden int }", map[string]string{"shared": "example.com/first"})
	outer := declare("example.com/outer", "Outer", "struct { *inner.Inner; Direct shared.Value; Values map[string]shared.Value; Fn func(shared.Value) error }", map[string]string{"inner": "example.com/inner", "shared": "example.com/second"})
	lookup := func(name string) (*x.Type, error) { return types[name], nil }
	shape := New(outer, lookup)
	for _, tc := range []struct{ path, want string }{
		{"Promoted", "*example.com/first.Value"}, {"Inner.Promoted", "*example.com/first.Value"},
		{"Promoted.Number", "int"}, {"Direct", "example.com/second.Value"}, {"Direct.Text", "string"},
		{"Values", "map[string]example.com/second.Value"}, {"Fn", "func(example.com/second.Value) error"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			result, err := shape.ResolveField(tc.path)
			if err != nil || result.Identity != tc.want {
				t.Fatalf("result=%+v err=%v want=%s", result, err, tc.want)
			}
		})
	}
	fields, err := shape.FieldsAt("Promoted")
	if err != nil || len(fields) != 1 || fields[0].Name != "Number" {
		t.Fatalf("promoted nested fields=%+v err=%v", fields, err)
	}
	for _, path := range []string{"", "missing", "hidden", "Inner.hidden", "Promoted.Text", "Direct..Text", "Direct. Text"} {
		if _, err := shape.ResolveField(path); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
	before, err := shape.Fields()
	if err != nil {
		t.Fatal(err)
	}
	second, err := shape.Fields()
	if err != nil || !reflect.DeepEqual(before, second) {
		t.Fatalf("equivalent fields changed equality: %v", err)
	}
	embedded.SynteticType.Imports["shared"].Path = "example.com/second"
	embedded.PkgPath = "example.com/changed"
	for _, field := range before {
		if field.Name != "Promoted" {
			continue
		}
		identity, err := field.CanonicalType()
		if err != nil || identity != "*example.com/first.Value" || field.TypeExpr != "*shared.Value" {
			t.Fatalf("retained field changed: %s %v %+v", identity, err, field)
		}
	}
	ambiguous := declare("example.com/outer", "Ambiguous", "struct { inner.Inner; Other }", map[string]string{"inner": "example.com/inner"})
	declare("example.com/outer", "Other", "struct { Promoted int }", nil)
	if _, err := New(ambiguous, lookup).ResolveField("Promoted"); err == nil {
		t.Fatal("accepted ambiguous promotion")
	}
}

type fieldResolutionGeneric[T any] struct {
	Value  T
	Values []*T
}

func TestFieldResolutionPreservesLinkedGenericIdentity(t *testing.T) {
	shape := Linked(reflect.TypeOf(fieldResolutionGeneric[Embedded]{}))
	for _, tc := range []struct{ path, want string }{
		{"Value", "github.com/viant/x/shape.Embedded"},
		{"Values", "[]*github.com/viant/x/shape.Embedded"},
		{"Value.ID", "int"},
	} {
		result, err := shape.ResolveField(tc.path)
		if err != nil || result.Identity != tc.want || result.Descriptor == nil {
			t.Fatalf("%s: %+v err=%v", tc.path, result, err)
		}
	}
}

func TestFieldResolutionSpecializedSyntheticGeneric(t *testing.T) {
	expression, err := parser.ParseExpr("struct { Value T; Values []*T }")
	if err != nil {
		t.Fatal(err)
	}
	descriptor := &x.Type{Name: "Box", PkgPath: "example.com/box", SynteticType: &model.Type{Name: "Box", PkgPath: "example.com/box", TypeSpec: &ast.TypeSpec{Name: ast.NewIdent("Box"), Type: expression, TypeParams: &ast.FieldList{List: []*ast.Field{{Names: []*ast.Ident{ast.NewIdent("T")}, Type: ast.NewIdent("any")}}}}}}
	lookup := func(name string) (*x.Type, error) {
		if name == "example.com/box.Box" {
			return descriptor, nil
		}
		return nil, nil
	}
	resolved, err := (Resolver{Lookup: lookup}).Resolve("example.com/box.Box[int]")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ path, want string }{{"Value", "int"}, {"Values", "[]*int"}} {
		result, err := New(resolved.Descriptor, lookup).ResolveField(tc.path)
		if err != nil || result.Identity != tc.want {
			t.Fatalf("%s: %+v err=%v", tc.path, result, err)
		}
	}
}

func TestFieldCanonicalTypeUsesSourceDeclarationPackage(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"example.com/declared", "example.com/declared.Value"},
		{"", "example.com/wrapper.Value"},
	} {
		expression, err := parser.ParseExpr("struct { Item Value }")
		if err != nil {
			t.Fatal(err)
		}
		descriptor := &x.Type{Name: "Wrapped", PkgPath: "example.com/wrapper", SynteticType: &model.Type{Name: "Wrapped", PkgPath: tc.source, TypeSpec: &ast.TypeSpec{Name: ast.NewIdent("Wrapped"), Type: expression}}}
		fields, err := New(descriptor, nil).Fields()
		if err != nil {
			t.Fatal(err)
		}
		canonical, err := fields[0].CanonicalType()
		if err != nil || canonical != tc.want {
			t.Fatalf("canonical=%s err=%v want=%s", canonical, err, tc.want)
		}
	}
}

package shape

import (
	"go/ast"
	"go/parser"
	"reflect"
	"testing"

	"github.com/viant/x"
	"github.com/viant/x/syntetic/model"
)

type Embedded struct{ ID int }
type shaped struct {
	Embedded
	Name  string `json:"name"`
	Child *Embedded
}

func TestRuntimeTypeMatrix(t *testing.T) {
	runtime := Runtime{Imports: map[string]string{"sample": "example.com/model"}, Lookup: func(name string) (reflect.Type, error) {
		if name == "example.com/model.Embedded" {
			return reflect.TypeOf(Embedded{}), nil
		}
		return nil, nil
	}}
	for _, tc := range []struct {
		source string
		want   reflect.Type
	}{
		{"map[string][]*sample.Embedded", reflect.TypeOf(map[string][]*Embedded{})},
		{"[2]int16", reflect.TypeOf([2]int16{})},
		{"chan<- int", reflect.TypeOf((chan<- int)(nil))},
		{"func(a,b int, rest ...string) (bool,error)", reflect.TypeOf((func(int, int, ...string) (bool, error))(nil))},
		{"struct { ID int; Name string `json:\"name\"` }", reflect.TypeOf(struct {
			ID   int
			Name string `json:"name"`
		}{})},
	} {
		t.Run(tc.source, func(t *testing.T) {
			actual, err := runtime.Type(tc.source)
			if err != nil || actual != tc.want {
				t.Fatalf("type=%v want=%v err=%v", actual, tc.want, err)
			}
		})
	}
	for _, source := range []string{"[bad]int", "map[[]int]int", "Missing", "struct { A int; A string }"} {
		if _, err := runtime.Type(source); err == nil {
			t.Fatalf("accepted invalid type %s", source)
		}
	}
}

func TestLinkedFieldPaths(t *testing.T) {
	shape := Linked(reflect.TypeOf(&shaped{}))
	fields, err := shape.Fields()
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 4 || fields[1].Name != "ID" || !reflect.DeepEqual(fields[1].Index, []int{0, 0}) {
		t.Fatalf("fields=%+v", fields)
	}
	field, err := shape.StructField("Child.ID")
	if err != nil || field.Type != reflect.TypeOf(0) || !reflect.DeepEqual(field.Index, []int{2, 0}) {
		t.Fatalf("field=%+v err=%v", field, err)
	}
	children, err := shape.FieldsAt("Child")
	if err != nil || len(children) != 1 || children[0].Name != "ID" {
		t.Fatalf("children=%+v err=%v", children, err)
	}
	if _, err := shape.StructField("Missing.ID"); err == nil {
		t.Fatal("missing field did not fail")
	}
}

func TestSyntheticGenericFields(t *testing.T) {
	expression, err := parser.ParseExpr("struct { Value T; Items []T }")
	if err != nil {
		t.Fatal(err)
	}
	declaration := &x.Type{Name: "Page", PkgPath: "example.com/model", SynteticType: &model.Type{Name: "Page", TypeSpec: &ast.TypeSpec{Name: ast.NewIdent("Page"), Type: expression, TypeParams: &ast.FieldList{List: []*ast.Field{{Names: []*ast.Ident{ast.NewIdent("T")}, Type: ast.NewIdent("any")}}}}}}
	lookup := func(name string) (*x.Type, error) {
		if name == "example.com/model.Page" {
			return declaration, nil
		}
		return nil, nil
	}
	resolved, err := (Resolver{Lookup: lookup, Imports: map[string]string{"model": "example.com/model"}}).Resolve("model.Page[int]")
	if err != nil {
		t.Fatal(err)
	}
	fields, err := New(resolved.Descriptor, lookup).Fields()
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 2 || fields[0].TypeExpr != "int" || fields[1].TypeExpr != "[]int" {
		t.Fatalf("fields=%+v", fields)
	}
	if rendered(declaration.SynteticType.TypeSpec.Type) != "struct {\n\tValue T\n\tItems []T\n}" {
		t.Fatal("generic declaration was mutated")
	}
}

func TestReferenceAndCanonicalMatrix(t *testing.T) {
	resolver := Resolver{Package: "example.com/local", Imports: map[string]string{"model": "example.com/model"}}
	for _, tc := range []struct{ source, want string }{
		{"[]*model.Page[int]", "[]*example.com/model.Page[int]"},
		{"map[string]*Local", "map[string]*example.com/local.Local"},
		{"example.com/model.Page[example.com/other.Item]", "example.com/model.Page[example.com/other.Item]"},
	} {
		actual, err := resolver.Canonical(tc.source)
		if err != nil || actual != tc.want {
			t.Fatalf("canonical=%s want=%s err=%v", actual, tc.want, err)
		}
	}
	reference, err := resolver.Reference("[]*model.Page[int]")
	if err != nil || reference.BaseName != "Page" || len(reference.Wrappers) != 2 || reference.Wrappers[0].Kind != WrapperSlice {
		t.Fatalf("reference=%+v err=%v", reference, err)
	}
	typeOf, err := (Runtime{}).Struct([]RuntimeField{{Name: "ID", TypeExpr: "int"}})
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := (Runtime{}).Synthetic("example.com/model", "Row", typeOf)
	if err != nil || descriptor.Type != typeOf || descriptor.SynteticType.TypeSpec == nil {
		t.Fatalf("descriptor=%+v err=%v", descriptor, err)
	}
}

func TestSyntheticGroupedAndInlineFields(t *testing.T) {
	expression, err := parser.ParseExpr("struct { A,B int; Nested []struct { Label string } }")
	if err != nil {
		t.Fatal(err)
	}
	descriptor := &x.Type{Name: "Outer", PkgPath: "example.com/model", SynteticType: &model.Type{TypeSpec: &ast.TypeSpec{Type: expression}}}
	shape := New(descriptor, nil)
	fields, err := shape.Fields()
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 3 || !reflect.DeepEqual(fields[1].Index, []int{1}) || !reflect.DeepEqual(fields[2].Index, []int{2}) {
		t.Fatalf("fields=%+v", fields)
	}
	nested, err := shape.FieldsAt("Nested")
	if err != nil || len(nested) != 1 || nested[0].TypeExpr != "string" {
		t.Fatalf("nested=%+v err=%v", nested, err)
	}
}

func TestCanonicalIgnoresTypeParenthesesAndGenericSpacing(t *testing.T) {
	resolver := Resolver{Package: "example.com/model"}
	for _, source := range []string{"*Row", "*(Row)", "(*Row)"} {
		got, err := resolver.Canonical(source)
		if err != nil || got != "*example.com/model.Row" {
			t.Fatalf("got=%s err=%v", got, err)
		}
	}
	got, err := resolver.Canonical("Pair[int, string]")
	if err != nil || got != "example.com/model.Pair[int,string]" {
		t.Fatalf("got=%s err=%v", got, err)
	}
}

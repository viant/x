package shape

import (
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func TestSourceUpdateStructFields(t *testing.T) {
	for _, test := range []struct {
		name, existing, generated, target, want, failure string
	}{
		{name: "pointer preserves exact bytes", existing: "package p\n// row\ntype Row struct {\n Z string // order\n // keep doc\n A int `json:\"custom\"` // keep comment\n}\nfunc (r Row) Custom() string { return r.Z }\n", generated: "package p;type Row struct { A *int `json:\"a\"`; Z string }", target: "*int", want: "package p\n// row\ntype Row struct {\n Z string // order\n // keep doc\n A *int `json:\"custom\"` // keep comment\n}\nfunc (r Row) Custom() string { return r.Z }\n"},
		{name: "reverse pointer", existing: "package p;type Row struct{ A *int }", generated: "package p;type Row struct{ A int }", target: "int", want: "package p;type Row struct{ A int }"},
		{name: "unlisted conflict", existing: "package p;type Row struct{ A int; B int }", generated: "package p;type Row struct{ A *int; B string }", target: "*int", failure: "Row.B has conflicting"},
		{name: "wrong exact target", existing: "package p;type Row struct{ A int }", generated: "package p;type Row struct{ A *int }", target: "string", failure: "exact generated type"},
		{name: "absent target", existing: "package p;type Row struct{ A int }", generated: "package p;type Row struct{ B *int }", target: "*int", failure: "no direct generated field"},
		{name: "grouped existing", existing: "package p;type Row struct{ A,B int }", generated: "package p;type Row struct{ A *int }", target: "*int", failure: "individually named field"},
		{name: "import alias reused", existing: "package p\nimport clock \"time\"\ntype Row struct { A clock.Time; Z clock.Time }", generated: "package p\nimport \"time\"\ntype Row struct{ A *time.Time; Z time.Time }", target: "*time.Time", want: "package p\nimport clock \"time\"\ntype Row struct { A *clock.Time; Z clock.Time }"},
		{name: "retired import with retained comments", existing: "package p\n// clock\nimport clock \"time\" // keep\ntype Row struct{ A clock.Time }", generated: "package p;type Row struct{ A int }", target: "int", want: "package p\n// clock\n // keep\ntype Row struct{ A int }"},
		{name: "unrelated import use retained", existing: "package p\nimport clock \"time\"\ntype Row struct{ A clock.Time };func Now() clock.Time {return clock.Now()}", generated: "package p;type Row struct{ A int }", target: "int", want: "package p\nimport clock \"time\"\ntype Row struct{ A int };func Now() clock.Time {return clock.Now()}"},
		{name: "same spelling different import identity", existing: "package p\nimport model \"example.com/old\"\ntype Row struct{ A model.Value }", generated: "package p\nimport model \"example.com/new\"\ntype Row struct{ A model.Value }", target: "model.Value", failure: "import alias model conflicts"},
		{name: "new import for exact type", existing: "package p\ntype Row struct{ A int }", generated: "package p\nimport \"time\"\ntype Row struct{ A *time.Time }", target: "*time.Time"},
		{name: "single line retired import", existing: "package p;import \"time\";type Row struct{ A time.Time }", generated: "package p;type Row struct{ A int }", target: "int"},
		{name: "grouped retired import", existing: "package p;import (\"time\");type Row struct{ A time.Time }", generated: "package p;type Row struct{ A int }", target: "int"},
		{name: "new field exact authority", existing: "package p;type Row struct{}", generated: "package p;type Row struct{ A *int }", target: "*int"},
		{name: "new type exact authority", existing: "package p;type Other struct{}", generated: "package p;type Row struct{ A *int }", target: "*int"},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := []byte(test.existing)
			generated := []byte(test.generated)
			updates := []SourceFieldTypeUpdate{{Owner: "Row", Field: "A", TypeExpr: test.target}}
			actual, err := (SourceParser{}).UpdateStructFields(original, generated, updates)
			if string(original) != test.existing || string(generated) != test.generated {
				t.Fatal("source mutated")
			}
			if test.failure != "" {
				if err == nil || !strings.Contains(err.Error(), test.failure) {
					t.Fatalf("error %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if test.want != "" && string(actual) != test.want {
				t.Fatalf("wanted %q got %q", test.want, actual)
			}
			again, err := (SourceParser{}).UpdateStructFields(actual, generated, updates)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actual, again) {
				t.Fatalf("not idempotent: %s", again)
			}
		})
	}
}

func TestSourceUpdateRetainsDefaultConflictProtection(t *testing.T) {
	old, new := []byte("package p;type Row struct{ A int }"), []byte("package p;type Row struct{ A *int }")
	if _, err := (SourceParser{}).AppendStructFields(old, new); err == nil {
		t.Fatal("default accepted type change")
	}
	if _, err := (SourceParser{}).UpdateStructFields(old, new, nil); err == nil {
		t.Fatal("empty authority accepted type change")
	}
	if _, err := (SourceParser{}).UpdateStructFields(old, new, []SourceFieldTypeUpdate{{"Row", "A", "*int"}, {"Row", "A", "string"}}); err == nil {
		t.Fatal("conflicting authority accepted")
	}
}

func TestSourceUpdateImportedTypesCompile(t *testing.T) {
	existing := []byte("package p\ntype Row struct{ A int }")
	generated := []byte("package p\nimport \"time\"\ntype Row struct{ A *time.Time }")
	updated, err := (SourceParser{}).UpdateStructFields(existing, generated, []SourceFieldTypeUpdate{{"Row", "A", "*time.Time"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, expression := range []string{"*time.Time", "int"} {
		if expression == "int" {
			updated, err = (SourceParser{}).UpdateStructFields(updated, existing, []SourceFieldTypeUpdate{{"Row", "A", "int"}})
			if err != nil {
				t.Fatal(err)
			}
		}
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, "row.go", string(updated)+"\nvar _ "+expression+" = Row{}.A\n", parser.AllErrors)
		if err != nil {
			t.Fatal(err)
		}
		config := types.Config{Importer: importer.Default()}
		if _, err = config.Check("example.com/p", set, []*ast.File{file}, nil); err != nil {
			t.Fatal(err)
		}
	}
}

package shape

import (
	"bytes"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestSourceAppendStructFields(t *testing.T) {
	for _, test := range []struct {
		name, existing, generated string
		want                      []string
		failure                   string
		noop                      bool
	}{
		{name: "preserve order and bytes", existing: "package p\n// authored\ntype Row struct {\n Z string // retain\n A int\n}\n", generated: "package p\ntype Row struct{ A int; B bool; Z string }", want: []string{"Z string // retain\n A int", "B bool"}},
		{name: "no op reordered fields", existing: "package p\ntype Row struct{ Z string; A int } // custom\n", generated: "package p\ntype Row struct{ A int; Z string }", noop: true},
		{name: "retain removed fields", existing: "package p\ntype Row struct{ A int; Z string }", generated: "package p\ntype Row struct{ A int }", noop: true},
		{name: "empty struct", existing: "package p\ntype Row struct{}", generated: "package p\ntype Row struct{ A int }", want: []string{"A int"}},
		{name: "type conflict", existing: "package p\ntype Row struct{ A int }", generated: "package p\ntype Row struct{ A string }", failure: "conflicting"},
		{name: "tag conflict", existing: "package p\ntype Row struct{ A int `json:\"a\"` }", generated: "package p\ntype Row struct{ A int `json:\"b\"` }", failure: "conflicting"},
		{name: "new import", existing: "package p // kept\ntype Row struct{ A int }", generated: "package p\nimport \"time\"\ntype Row struct{ A int; At time.Time }", want: []string{"package p // kept", "import time \"time\"", "At time.Time"}},
		{name: "existing import alias", existing: "package p\nimport clock \"time\"\ntype Row struct{ At clock.Time }", generated: "package p\nimport \"time\"\ntype Row struct{ At time.Time; Later time.Time }", want: []string{"Later clock.Time"}},
		{name: "new type", existing: "package p\ntype Row struct{ A int }", generated: "package p\ntype Row struct{ A int };type Child struct{ B string }", want: []string{"type Child struct{ B string }"}},
		{name: "group partial overlap", existing: "package p\ntype Row struct{ A int }", generated: "package p\ntype Row struct{ A,B int }", failure: "partially overlaps"},
		{name: "anonymous field identity", existing: "package p\ntype Row struct{ *Base };type Base struct{}", generated: "package p\ntype Row struct{ *Base; Next string };type Base struct{}", want: []string{"Next string"}},
		{name: "duplicate existing type", existing: "package p;type Row struct{};type Row struct{}", generated: "package p;type Row struct{}", failure: "duplicate existing"},
		{name: "duplicate generated type", existing: "package p;type Row struct{}", generated: "package p;type Row struct{};type Row struct{}", failure: "duplicate generated"},
		{name: "duplicate generated field", existing: "package p;type Row struct{}", generated: "package p;type Row struct{ A int;A int }", failure: "duplicate generated"},
		{name: "generic constraint alias", existing: "package p\nimport clock \"example.com/constraint\"\ntype Row[T clock.Value] struct{ A T }", generated: "package p\nimport \"example.com/constraint\"\ntype Row[T constraint.Value] struct{ A T; B string }", want: []string{"B string"}},
		{name: "preserve custom method", existing: "package p\ntype Row struct{ A int };func(r *Row) Custom() int{return r.A}", generated: "package p\ntype Row struct{ A int;B string }", want: []string{"func(r *Row) Custom() int{return r.A}"}},
		{name: "unresolved default package qualifier", existing: "package p;type Row struct{}", generated: "package p\nimport \"example.com/foo/v2\"\ntype Row struct{ Value foo.Value }", failure: "explicit import alias"},
		{name: "explicit versioned package qualifier", existing: "package p;type Row struct{}", generated: "package p\nimport foo \"example.com/foo/v2\"\ntype Row struct{ Value foo.Value }", want: []string{"import foo \"example.com/foo/v2\"", "Value foo.Value"}},
		{name: "type conflicts with existing function", existing: "package p;func Row(){}", generated: "package p;type Row struct{}", failure: "existing package declaration"},
	} {
		t.Run(test.name, func(t *testing.T) {
			actual, err := (SourceParser{}).AppendStructFields([]byte(test.existing), []byte(test.generated))
			if test.failure != "" {
				if err == nil || !strings.Contains(err.Error(), test.failure) {
					t.Fatalf("error %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if test.noop && !bytes.Equal(actual, []byte(test.existing)) {
				t.Fatalf("no-op rewrote source: %s", actual)
			}
			for _, wanted := range test.want {
				if !strings.Contains(string(actual), wanted) {
					t.Fatalf("missing %q in %s", wanted, actual)
				}
			}
			if _, err = parser.ParseFile(token.NewFileSet(), "merged.go", actual, parser.AllErrors); err != nil {
				t.Fatal(err)
			}
			again, err := (SourceParser{}).AppendStructFields(actual, []byte(test.generated))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actual, again) {
				t.Fatalf("merge not idempotent: %s", again)
			}
		})
	}
}

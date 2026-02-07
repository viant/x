package model

import (
	"go/ast"
	"testing"
)

func TestImportSetAliasFor(t *testing.T) {
	im := NewImportSet("github.com/viant/x/current")

	tests := []struct {
		name    string
		pkgPath string
		want    string
	}{
		{"empty", "", ""},
		{"current", "github.com/viant/x/current", ""},
		{"simple", "github.com/viant/foo", "foo"},
		{"dash sanitization", "example.com/my-pkg", "my_pkg"},
	}

	for _, tt := range tests {
		got := im.AliasFor(tt.pkgPath)
		if got != tt.want {
			t.Errorf("%s: AliasFor(%q) = %q, want %q", tt.name, tt.pkgPath, got, tt.want)
		}
	}

	// conflict resolution
	first := im.AliasFor("example.com/a/foo")
	second := im.AliasFor("example.com/b/foo")
	if first == second {
		t.Fatalf("expected distinct aliases for conflicting paths, got %q", first)
	}
}

func TestImportSetQualIdent(t *testing.T) {
	im := NewImportSet("github.com/viant/x/current")

	// local package should not be qualified
	if expr, ok := im.QualIdent("github.com/viant/x/current", "Type").(*ast.Ident); !ok || expr.Name != "Type" {
		t.Fatalf("expected local ident, got %#v", expr)
	}

	// external package should be qualified
	expr, ok := im.QualIdent("github.com/viant/foo", "Bar").(*ast.SelectorExpr)
	if !ok {
		t.Fatalf("expected selector expression, got %#v", expr)
	}
	if ident, ok := expr.X.(*ast.Ident); !ok || ident.Name == "" {
		t.Fatalf("expected non-empty alias ident, got %#v", expr.X)
	}
}

func TestImportSetEntriesDeterministic(t *testing.T) {
	im := NewImportSet("github.com/viant/x/current")

	paths := []string{
		"github.com/viant/a/foo",
		"github.com/viant/b/foo",
		"github.com/viant/c/bar",
	}
	for _, p := range paths {
		_ = im.AliasFor(p)
	}

	entries1 := im.Entries()
	entries2 := im.Entries()
	if len(entries1) != len(entries2) {
		t.Fatalf("entries length mismatch: %d vs %d", len(entries1), len(entries2))
	}
	for i := range entries1 {
		if entries1[i] != entries2[i] {
			t.Fatalf("entries not deterministic at index %d: %#v vs %#v", i, entries1[i], entries2[i])
		}
	}
}

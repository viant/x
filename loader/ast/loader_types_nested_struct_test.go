package ast

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"testing"

	"github.com/viant/x/syntetic/model"
)

func TestLoadPackageFS_Types_NestedStructs(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/nested\n\n",
		"root/p/nested.go": `package p

type Inner struct{ V int }

type Outer struct {
    In     Inner
    Inline struct{ S string; Ns []int }
    List   []Inner
    Matrix [][]int
    Dict   map[string]*Inner
    Mixed  map[string][]*Inner
}
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	if !pkg.HasType("Inner") || !pkg.HasType("Outer") {
		t.Fatalf("expected Inner and Outer types; got %d", len(pkg.Types))
	}
	outer := findTypeN(pkg, "Outer")
	if outer == nil {
		t.Fatalf("type Outer not found")
	}

	// Compare struct body with expected nested shape using AST normalization.
	expected := `struct{
        In Inner
        Inline struct{ S string; Ns []int }
        List []Inner
        Matrix [][]int
        Dict map[string]*Inner
        Mixed map[string][]*Inner
    }`
	assertTypeBodyASTEqualEx(t, outer.Body(), expected)
}

func findTypeN(p *model.Package, name string) *model.Type {
	for _, t := range p.Types {
		if t != nil && t.Name == name {
			return t
		}
	}
	return nil
}

func assertTypeBodyASTEqualEx(t *testing.T, rendered, expectedExpr string) {
	t.Helper()
	parse := func(expr string) ast.Expr {
		node, err := parser.ParseExpr(expr)
		if err != nil {
			t.Fatalf("failed to parse expression %q: %v", expr, err)
		}
		typed, ok := node.(ast.Expr)
		if !ok {
			t.Fatalf("parsed node for %q is not an ast.Expr", expr)
		}
		return typed
	}
	format := func(expr ast.Expr) string {
		var fs token.FileSet
		var buf bytes.Buffer
		_ = printer.Fprint(&buf, &fs, expr)
		return buf.String()
	}
	got := format(parse(rendered))
	want := format(parse(expectedExpr))
	if got != want {
		t.Fatalf("AST mismatch:\n got: %s\nwant: %s", got, want)
	}
}

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

func TestLoadPackageFS_Types_Structs(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/types\n\n",
		"root/p/structs.go": `package p

type S1 struct{ ID int; Name string }

type T struct{ X int }

type S2 struct{
    A []int
    B map[string]*T
}

type Emb struct{ S1 }
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	if !pkg.HasType("S1") || !pkg.HasType("S2") || !pkg.HasType("Emb") || !pkg.HasType("T") {
		t.Fatalf("missing expected struct types; got %d", len(pkg.Types))
	}
	// Validate body AST for S1
	s1 := findType(pkg, "S1")
	if s1 == nil {
		t.Fatalf("type S1 not found")
	}
	assertTypeBodyASTEqual(t, s1.Body(), "struct{ ID int; Name string }")
}

func findType(p *model.Package, name string) *model.Type {
	for _, t := range p.Types {
		if t != nil && t.Name == name {
			return t
		}
	}
	return nil
}

func assertTypeBodyASTEqual(t *testing.T, rendered, expectedExpr string) {
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
	r := parse(rendered)
	e := parse(expectedExpr)
	// Normalize both ASTs via printer and compare strings.
	got := formatExpr(r)
	want := formatExpr(e)
	if got != want {
		t.Fatalf("AST mismatch:\n got: %s\nwant: %s", got, want)
	}
}

func formatExpr(expr ast.Expr) string {
	var fs token.FileSet
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, &fs, expr)
	return buf.String()
}

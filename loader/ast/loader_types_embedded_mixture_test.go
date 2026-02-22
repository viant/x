package ast

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"testing"
)

func TestLoadPackageFS_Types_EmbeddedMixture(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/mix\n\n",
		"root/p/mix.go": `package p

type Base struct{ ID int }

type Mixin struct{ Z string }

type S struct{
    Base
    *Mixin
    Name string
    Ages []int
}
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	if !pkg.HasType("S") {
		t.Fatalf("expected type S")
	}
	s := findTypeG(pkg, "S")
	expected := `struct{ Base; *Mixin; Name string; Ages []int }`
	assertTypeBodyASTEqualMix(t, s.Body(), expected)
}

func assertTypeBodyASTEqualMix(t *testing.T, rendered, expectedExpr string) {
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
	if format(parse(rendered)) != format(parse(expectedExpr)) {
		t.Fatalf("AST mismatch:\n got: %s\nwant: %s", rendered, expectedExpr)
	}
}

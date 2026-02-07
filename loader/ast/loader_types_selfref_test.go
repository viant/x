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

func TestLoadPackageFS_Types_SelfRef(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod":    "module example.com/self\n\n",
		"root/p/self.go": "package p\n\ntype T struct{ id int; slice []T }\n",
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	if !pkg.HasType("T") {
		t.Fatalf("expected type T")
	}
	tpe := findTypeSelf(pkg, "T")
	if tpe == nil {
		t.Fatalf("type T not found")
	}
	assertBodyEquals(t, tpe.Body(), "struct{ id int; slice []T }")
}

func findTypeSelf(p *model.Package, name string) *model.Type {
	for _, t := range p.Types {
		if t != nil && t.Name == name {
			return t
		}
	}
	return nil
}

func assertBodyEquals(t *testing.T, rendered, expected string) {
	t.Helper()
	parse := func(expr string) ast.Expr {
		node, err := parser.ParseExpr(expr)
		if err != nil {
			t.Fatalf("failed to parse expression %q: %v", expr, err)
		}
		e, _ := node.(ast.Expr)
		return e
	}
	norm := func(e ast.Expr) string {
		var fs token.FileSet
		var buf bytes.Buffer
		_ = printer.Fprint(&buf, &fs, e)
		return buf.String()
	}
	if norm(parse(rendered)) != norm(parse(expected)) {
		t.Fatalf("body mismatch:\n got: %s\nwant: %s", rendered, expected)
	}
}

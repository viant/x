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

func TestLoadPackageFS_Types_Generics(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/types\n\n",
		"root/p/generics.go": `package p

type Box[T any] struct{ V T }
type Pair[K comparable, V any] struct{ K K; V V }
type SliceOf[T any] []T
type MapOf[K comparable, V any] map[K]V
type Alias[T any] = []T
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	// Ensure types are present
	for _, n := range []string{"Box", "Pair", "SliceOf", "MapOf", "Alias"} {
		if !pkg.HasType(n) {
			t.Fatalf("missing type %s", n)
		}
	}
	// Check type parameters captured on type declarations
	box := findTypeG(pkg, "Box")
	if len(box.TypeParams) != 1 || box.TypeParams[0].Name != "T" {
		t.Fatalf("expected Box to have [T] type param, got %#v", box.TypeParams)
	}
	pair := findTypeG(pkg, "Pair")
	if len(pair.TypeParams) != 2 || pair.TypeParams[0].Name != "K" || pair.TypeParams[1].Name != "V" {
		t.Fatalf("expected Pair[K,V], got %#v", pair.TypeParams)
	}
	// Validate bodies reference type parameters appropriately.
	assertTypeBodyASTEqualG(t, findTypeG(pkg, "Box").Body(), "struct{ V T }")
	assertTypeBodyASTEqualG(t, findTypeG(pkg, "Pair").Body(), "struct{ K K; V V }")
	assertTypeBodyASTEqualG(t, findTypeG(pkg, "SliceOf").Body(), "[]T")
	assertTypeBodyASTEqualG(t, findTypeG(pkg, "MapOf").Body(), "map[K]V")
	// Alias body should print underlying expression
	assertTypeBodyASTEqualG(t, findTypeG(pkg, "Alias").Body(), "[]T")
}

func findTypeG(p *model.Package, name string) *model.Type {
	for _, t := range p.Types {
		if t != nil && t.Name == name {
			return t
		}
	}
	return nil
}

func assertTypeBodyASTEqualG(t *testing.T, rendered, expectedExpr string) {
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

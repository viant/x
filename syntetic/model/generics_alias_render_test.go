package model

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"testing"
)

func TestType_ToGenDecl_AliasWithTypeParams(t *testing.T) {
	// type Alias[T any] = []T
	ts := &ast.TypeSpec{
		Name:   ast.NewIdent("Alias"),
		Type:   &ast.ArrayType{Elt: ast.NewIdent("T")},
		Assign: token.Pos(1), // non-zero indicates alias during printing
	}
	tpe := &Type{
		Name:       "Alias",
		PkgPath:    "example.com/p",
		TypeParams: []TypeParam{{Name: "T", Constraint: &Basic{Name: "any"}}},
		TypeSpec:   ts,
	}
	decl := tpe.ToGenDecl("example.com/p", nil)
	if decl == nil {
		t.Fatalf("expected decl")
	}
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, token.NewFileSet(), decl)
	out := buf.String()
	if !contains(out, "type Alias[T any] = []T") {
		t.Fatalf("expected alias with type params, got: %s", out)
	}
}

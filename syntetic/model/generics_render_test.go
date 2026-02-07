package model

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"testing"
)

func TestType_ToGenDecl_WithTypeParams(t *testing.T) {
	// type Box[T any] struct{ V T }
	tpe := &Type{
		Name:       "Box",
		PkgPath:    "example.com/p",
		TypeParams: []TypeParam{{Name: "T", Constraint: &Basic{Name: "any"}}},
		TypeSpec:   &ast.TypeSpec{Name: ast.NewIdent("Box"), Type: &ast.StructType{Fields: &ast.FieldList{List: []*ast.Field{{Names: []*ast.Ident{ast.NewIdent("V")}, Type: ast.NewIdent("T")}}}}},
	}
	decl := tpe.ToGenDecl("example.com/p", nil)
	if decl == nil {
		t.Fatalf("expected decl")
	}
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, token.NewFileSet(), decl)
	src := buf.String()
	if !contains(src, "type Box[T any]") || !contains(src, "struct") {
		t.Fatalf("expected generic declaration, got: %s", src)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(s) > len(sub) && indexOf(s, sub) >= 0))
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

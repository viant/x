package model

import (
	"go/ast"
	"go/parser"
	"strings"
	"testing"
)

func TestGoFile_RenderGlobals(t *testing.T) {
	gf := &GoFile{PkgName: "example"}

	// const A = 1
	valA, _ := parser.ParseExpr("1")
	gf.AddConst(ConstDecl{Name: "A", Value: valA})

	// const B string = "x"
	typeB, _ := parser.ParseExpr("string")
	valB, _ := parser.ParseExpr("\"x\"")
	gf.AddConst(ConstDecl{Name: "B", Type: typeB, Value: valB})

	// var X int
	typeX, _ := parser.ParseExpr("int")
	gf.AddVar(VarDecl{Name: "X", Type: typeX})

	// var Y = 2
	valY, _ := parser.ParseExpr("2")
	gf.AddVar(VarDecl{Name: "Y", Value: valY})

	// type T struct{ N int }
	exprT, _ := parser.ParseExpr("struct{ N int }")
	gf.AddType(&Type{Name: "T", TypeSpec: &ast.TypeSpec{Name: ast.NewIdent("T"), Type: exprT}})

	src, err := gf.Render()
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if !(strings.Contains(src, "const (") && strings.Contains(src, "var (") && strings.Contains(src, "type T ")) {
		t.Fatalf("rendered source missing expected sections:\n%s", src)
	}
}

package model

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
)

// ExampleGoFile_Render demonstrates rendering a simple file containing a type
// declaration using AST-based emission.
func ExampleGoFile_Render() {
	expr, _ := parser.ParseExpr("struct{ ID int }")
	t := &Type{Name: "T", TypeSpec: &ast.TypeSpec{Name: ast.NewIdent("T"), Type: expr}}
	gf := &GoFile{PkgName: "example", Types: []*Type{t}}
	src, _ := gf.Render()
	fmt.Println(strings.HasPrefix(src, "package example"))
	// Output: true
}

// ExamplePackage_RenderFilesWithPackageOptions shows how to render files with
// interleaved method stubs at the package level.
func ExamplePackage_RenderFilesWithPackageOptions() {
	// Build a minimal method AST for demonstration.
	fdSrc := "package p\nfunc (a A) M(){}\n"
	f, _ := parser.ParseFile(token.NewFileSet(), "m.go", fdSrc, 0)
	fd := f.Decls[0].(*ast.FuncDecl)

	a := &Type{Name: "A", TypeSpec: &ast.TypeSpec{Name: ast.NewIdent("A"), Type: ast.NewIdent("struct{}")}, MethodsAST: []*ast.FuncDecl{fd}}
	pkg := &Package{Name: "p", Files: []*GoFile{{Name: "a.go", PkgName: "p", Types: []*Type{a}}}}
	out, _ := pkg.RenderFilesWithPackageOptions(PackageRenderOptions{Render: RenderOptions{InterleaveMethodStubs: true}})
	fmt.Println(strings.Contains(out["a.go"], "func (a A) M()"))
	// Output: true
}

// ExampleType_ToGenDecl_generics shows how a Type with TypeParams and an
// alias render via ToGenDecl.
func ExampleType_ToGenDecl_generics() {
	// type Box[T any] struct{ V T }
	box := &Type{
		Name:       "Box",
		PkgPath:    "example.com/p",
		TypeParams: []TypeParam{{Name: "T", Constraint: &Basic{Name: "any"}}},
		TypeSpec: &ast.TypeSpec{
			Name: ast.NewIdent("Box"),
			Type: &ast.StructType{Fields: &ast.FieldList{List: []*ast.Field{{Names: []*ast.Ident{ast.NewIdent("V")}, Type: ast.NewIdent("T")}}}},
		},
	}
	// type IDs[T any] = []T
	alias := &Type{
		Name:       "IDs",
		PkgPath:    "example.com/p",
		TypeParams: []TypeParam{{Name: "T", Constraint: &Basic{Name: "any"}}},
		TypeSpec:   &ast.TypeSpec{Name: ast.NewIdent("IDs"), Type: &ast.ArrayType{Elt: ast.NewIdent("T")}, Assign: token.Pos(1)},
	}
	var buf1, buf2 strings.Builder
	_ = printer.Fprint(&buf1, token.NewFileSet(), box.ToGenDecl("example.com/p", nil))
	_ = printer.Fprint(&buf2, token.NewFileSet(), alias.ToGenDecl("example.com/p", nil))
	fmt.Println(strings.Contains(buf1.String(), "type Box[T any]"))
	fmt.Println(strings.Contains(buf2.String(), "type IDs[T any] = []T"))
	// Output:
	// true
	// true
}

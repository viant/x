package model

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestPackage_WriteFilesWithOptions(t *testing.T) {
	p := &Package{
		Name:  "p",
		Files: []*GoFile{{Name: "a.go", PkgName: "p", Types: []*Type{{Name: "T", TypeSpec: mustTypeSpec("T", "struct{}")}}}},
	}
	dir := t.TempDir()
	if err := p.WriteFilesWithOptions(dir, RenderOptions{InterleaveMethodStubs: true}, false); err != nil {
		t.Fatalf("WriteFilesWithOptions error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.go")); err != nil {
		t.Fatalf("expected a.go to be written: %v", err)
	}
}

func TestPackage_RenderFilesWithPackageOptions_InterleaveOverride(t *testing.T) {
	// Two files, each with a type + method; ensure interleave option applies
	// globally and can be overridden per file.
	// Build a simple FuncDecl for method stubs by parsing a tiny file.
	parseFunc := func(src string) *ast.FuncDecl {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "x.go", src, 0)
		if err != nil || len(f.Decls) == 0 {
			return nil
		}
		if fd, ok := f.Decls[0].(*ast.FuncDecl); ok {
			return fd
		}
		return nil
	}
	// Methods for A and B
	mA := parseFunc("package p\nfunc (a A) M(){}\n")
	mB := parseFunc("package p\nfunc (b B) N(){}\n")
	if mA == nil || mB == nil {
		t.Fatal("failed to build func decls")
	}
	a := &Type{Name: "A", TypeSpec: mustTypeSpec("A", "struct{}"), MethodsAST: []*ast.FuncDecl{mA}}
	b := &Type{Name: "B", TypeSpec: mustTypeSpec("B", "struct{}"), MethodsAST: []*ast.FuncDecl{mB}}
	p := &Package{
		Name:  "p",
		Files: []*GoFile{{Name: "a.go", PkgName: "p", Types: []*Type{a}}, {Name: "b.go", PkgName: "p", Types: []*Type{b}}},
	}
	po := PackageRenderOptions{Render: RenderOptions{InterleaveMethodStubs: true}}
	files, err := p.RenderFilesWithPackageOptions(po)
	if err != nil {
		t.Fatalf("RenderFilesWithPackageOptions error: %v", err)
	}
	if !contains(files["a.go"], "func (a A) M()") || !contains(files["b.go"], "func (b B) N()") {
		t.Fatalf("expected interleaved stubs in both files; got:\nA:\n%s\nB:\n%s", files["a.go"], files["b.go"])
	}
}

func mustTypeSpec(name, body string) *ast.TypeSpec {
	expr, _ := parser.ParseExpr(body)
	return &ast.TypeSpec{Name: ast.NewIdent(name), Type: expr}
}

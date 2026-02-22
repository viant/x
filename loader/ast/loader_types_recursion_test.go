package ast

import (
	"context"
	"go/parser"
	"go/token"
	"testing"
)

func TestPointerRecursion_Node(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod":    "module example.com/rec\n\n",
		"root/p/node.go": "package p\n\n type Node struct{ Next *Node }\n",
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	if !pkg.HasType("Node") {
		t.Fatalf("expected type Node present")
	}
	files, err := pkg.RenderFilesWithMethods()
	if err != nil {
		t.Fatalf("RenderFilesWithMethods error: %v", err)
	}
	src := files["node.go"]
	if src == "" || !contains(src, "type Node struct") || !contains(src, "*Node") {
		t.Fatalf("expected rendered recursive pointer type; got:\n%s", src)
	}
	// e2e parse rendered file
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "node.go", src, 0); err != nil {
		t.Fatalf("parsed rendered file with error: %v", err)
	}
}

func TestMutualRecursion_AB(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/mut\n\n",
		"root/p/a.go": "package p\n\n type A struct{ B *B }\n",
		"root/p/b.go": "package p\n\n type B struct{ A *A }\n",
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	if !pkg.HasType("A") || !pkg.HasType("B") {
		t.Fatalf("expected types A and B present")
	}
	files, err := pkg.RenderFilesWithMethods()
	if err != nil {
		t.Fatalf("RenderFilesWithMethods error: %v", err)
	}
	srcA := files["a.go"]
	srcB := files["b.go"]
	if !contains(srcA, "type A struct") || !contains(srcA, "*B") {
		t.Fatalf("expected A to reference *B; got:\n%s", srcA)
	}
	if !contains(srcB, "type B struct") || !contains(srcB, "*A") {
		t.Fatalf("expected B to reference *A; got:\n%s", srcB)
	}
	// e2e parse rendered files
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "a.go", srcA, 0); err != nil {
		t.Fatalf("parse a.go: %v", err)
	}
	if _, err := parser.ParseFile(fset, "b.go", srcB, 0); err != nil {
		t.Fatalf("parse b.go: %v", err)
	}
}

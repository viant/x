package ast

import (
	"context"
	"testing"
)

func TestConstraints_UnionWithSelectorAndApprox(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod":           "module example.com/cns\n\n",
		"root/p/constraints.go": "package p\n\nimport \"example.com/other\"\n\n type C[T interface{ ~int | other.S | *other.S }] struct{}\n",
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	files, err := pkg.RenderFilesWithMethods()
	if err != nil {
		t.Fatalf("RenderFilesWithMethods error: %v", err)
	}
	src := files["constraints.go"]
	if !(contains(src, "type C[T interface ") && contains(src, "~int") && contains(src, "other.S") && contains(src, "*other.S")) {
		t.Fatalf("expected union constraint with approx and selector; got:\n%s", src)
	}
}

func TestConstraints_NestedParenUnion(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod":           "module example.com/cns2\n\n",
		"root/p/constraints.go": "package p\n\n type D[T interface{ int | string | int }] struct{}\n",
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	files, err := pkg.RenderFilesWithMethods()
	if err != nil {
		t.Fatalf("RenderFilesWithMethods error: %v", err)
	}
	src := files["constraints.go"]
	if !(contains(src, "type D[T interface ") && contains(src, "string")) {
		t.Fatalf("expected nested union constraint; got:\n%s", src)
	}
}

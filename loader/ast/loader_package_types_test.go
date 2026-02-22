package ast

import (
	"context"
	"testing"
)

func TestLoadPackageFS_Types(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod":          "module example.com/mod\n\n",
		"root/p/types.go":      "package p\n\n type T struct{ N int }\n type S []string\n",
		"root/p/other_test.go": "package p\n\nfunc TestX(t *testing.T) {}\n",
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	if pkg.Name != "p" || pkg.PkgPath != "example.com/mod/p" {
		t.Fatalf("unexpected package meta: name=%s path=%s", pkg.Name, pkg.PkgPath)
	}
	if !pkg.HasType("T") || !pkg.HasType("S") {
		t.Fatalf("expected types T and S; got %d types", len(pkg.Types))
	}
}

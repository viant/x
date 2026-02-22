package ast

import (
	"context"
	"testing"
)

func TestLoadPackageFS_MultiFile(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module ex.com/m\n\n",
		"root/p/a.go": "package p\n type A struct{}\n",
		"root/p/b.go": "package p\n type B struct{}\n",
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	if !pkg.HasType("A") || !pkg.HasType("B") || len(pkg.Files) != 2 {
		t.Fatalf("expected 2 types across 2 files; types=%d files=%d", len(pkg.Types), len(pkg.Files))
	}
}

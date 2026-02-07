package ast

import (
	"context"
	"testing"
)

func TestLoadPackageFS_ImportsAliasing(t *testing.T) {
	src := "package p\n\nimport \"time\"\nimport ctx \"context\"\nvar A = time.Second\nvar B ctx.Context\n"
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/m\n\n",
		"root/p/a.go": src,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	gf := pkg.Files[0]
	if !gf.HasImport("time", "") || !gf.HasImport("context", "ctx") {
		t.Fatalf("expected time and aliased ctx imports in file")
	}
}

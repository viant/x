package ast

import (
	"context"
	"testing"
)

func TestLoadPackageFS_Globals(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod":    "module example.com/m\n\n",
		"root/p/file.go": "package p\n\nimport tm \"time\"\nconst A = tm.Second\nvar X = tm.Now()\n",
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	if !pkg.HasConst("A") || !pkg.HasVar("X") {
		t.Fatalf("expected const A and var X; consts=%d vars=%d", len(pkg.Consts), len(pkg.Vars))
	}
	// Ensure per-decl imports captured
	gf := pkg.Files[0]
	if !gf.HasConst("A") || !gf.HasVar("X") {
		t.Fatalf("expected file to include A and X as globals")
	}
}

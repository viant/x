package ast

import (
	"context"
	"testing"
)

func TestGoFile_AddSideEffectImport(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/se\n\n",
		"root/p/a.go": "package p\n\n type T struct{}\n",
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	gf := pkg.FileByName("a.go")
	if gf == nil {
		t.Fatalf("expected file a.go")
	}
	gf.AddSideEffectImport("example.com/driver")
	src, err := gf.Render()
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !contains(src, "_ \"example.com/driver\"") {
		t.Fatalf("expected side-effect import in output:\n%s", src)
	}
}

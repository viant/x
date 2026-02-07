package ast

import (
	"context"
	"strings"
	"testing"
)

func TestGoFile_RenderWithMethods(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/methods\n\n",
		"root/p/t.go": `package p

type T struct{ X int }

func (t T) Val() int { return t.X }
func (t *T) Ptr() error { return nil }
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	if !pkg.HasType("T") {
		t.Fatalf("expected type T")
	}
	gf := pkg.FileByName("t.go")
	if gf == nil {
		t.Fatalf("expected file t.go")
	}
	if len(gf.Types) == 0 {
		t.Fatalf("file has no types")
	}
	if len(gf.Types[0].MethodsAST) == 0 && len(gf.Types[0].PtrMethodsAST) == 0 {
		t.Fatalf("expected methods attached to type in file; got 0")
	}
	stubs := gf.Types[0].MethodStubs()
	if len(stubs) == 0 {
		t.Fatalf("expected MethodStubs non-empty on file type")
	}
	src, err := gf.RenderWithMethods()
	if err != nil {
		t.Fatalf("RenderWithMethods error: %v", err)
	}
	if !strings.Contains(src, "type T struct") {
		t.Fatalf("expected type declaration in output:\n%s", src)
	}
	if !strings.Contains(src, "func (t T) Val()") || !strings.Contains(src, "func (t *T) Ptr()") {
		t.Fatalf("expected method stubs in output; stubs=%q\n%s", strings.Join(stubs, " | "), src)
	}
}

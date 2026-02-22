package ast

import (
	"context"
	"strings"
	"testing"

	"github.com/viant/x/syntetic/model"
)

func TestGoFile_RenderWithOptions_Interleave(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/inter\n\n",
		"root/p/t.go": `package p

type A struct{}
func (a A) M() {}

type B struct{}
func (b B) N() {}
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	gf := pkg.FileByName("t.go")
	if gf == nil {
		t.Fatalf("expected file t.go")
	}
	src, err := gf.RenderWithOptions(model.RenderOptions{InterleaveMethodStubs: true})
	if err != nil {
		t.Fatalf("RenderWithOptions error: %v", err)
	}
	iTypeA := strings.Index(src, "type A struct")
	iMethA := strings.Index(src, "func (a A) M()")
	iTypeB := strings.Index(src, "type B struct")
	iMethB := strings.Index(src, "func (b B) N()")
	if !(iTypeA >= 0 && iMethA > iTypeA && iTypeB > iMethA && iMethB > iTypeB) {
		t.Fatalf("expected interleaved order: type A, M, type B, N; got:\n%s", src)
	}
}

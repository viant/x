package ast

import (
	"context"
	"strings"
	"testing"
)

func TestMethodStub_WithGenericReceiverTypeArg(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/mg\n\n",
		"root/p/t.go": `package p

type Box[T any] struct{}

func (b Box[T]) M(u T) T { var z T; return z }
`,
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
	src := files["t.go"]
	if !strings.Contains(src, "type Box[T any]") {
		t.Fatalf("expected generic type in output:\n%s", src)
	}
	if !strings.Contains(src, "func (b Box[T]) M(u T) T") {
		t.Fatalf("expected method stub with generic receiver type arg; got:\n%s", src)
	}
}

func TestFunctionStub_WithTypeParams(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/fg\n\n",
		"root/p/f.go": `package p

func F[U any](u U) U { var z U; return z }
`,
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
	src := files["f.go"]
	if !strings.Contains(src, "func F[U any](u U) U") {
		t.Fatalf("expected generic function stub; got:\n%s", src)
	}
}

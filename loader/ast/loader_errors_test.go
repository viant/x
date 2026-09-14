package ast

import (
	"context"
	"errors"
	"go/build"
	"testing"
)

func TestLoadPackageFS_Errors(t *testing.T) {
	ctx := context.Background()
	// No go files
	fsys := MkFS(t, map[string]string{"root/go.mod": "module ex.com/m\n"})
	if _, err := LoadPackageFS(ctx, fsys, "root"); err == nil {
		t.Fatalf("expected error for directory with no go files")
	} else {
		var noGo *build.NoGoError
		if !errors.As(err, &noGo) || noGo.Dir != "root" {
			t.Fatalf("empty package error = %T %[1]v", err)
		}
	}
	// No go.mod
	fsys2 := MkFS(t, map[string]string{"root/p/a.go": "package p\n"})
	if _, err := LoadPackageFS(ctx, fsys2, "root/p"); err == nil {
		t.Fatalf("expected error for missing go.mod")
	}
}

package ast

import (
	"context"
	"testing"
)

func TestLoadModuleFS_Discovery(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod":        "module example.com/m\n\n",
		"root/a/a.go":        "package a\n type A struct{}\n",
		"root/b/b.go":        "package b\n type B struct{}\n",
		"root/vendor/x/x.go": "package x\n type X struct{}\n",
	})
	ctx := context.Background()
	mod, err := LoadModuleFS(ctx, fsys, "root")
	if err != nil {
		t.Fatalf("LoadModuleFS error: %v", err)
	}
	if !mod.HasPackage("example.com/m/a") || !mod.HasPackage("example.com/m/b") {
		t.Fatalf("expected packages a and b discovered")
	}
	if mod.HasPackage("example.com/m/vendor/x") {
		t.Fatalf("vendor directory should be skipped")
	}
}

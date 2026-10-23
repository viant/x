package ast

import (
	"context"
	"testing"
	"testing/fstest"
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

func TestLoadModuleFSSkipsNestedModuleAndPropagatesParseError(t *testing.T) {
	fsys := fstest.MapFS{
		"root/go.mod":                {Data: []byte("module example.com/root\n")},
		"root/app/app.go":            {Data: []byte("package app\ntype App struct{}\n")},
		"root/nested/go.mod":         {Data: []byte("module example.com/root/nested\n")},
		"root/nested/child/child.go": {Data: []byte("package child\ntype Child struct{}\n")},
	}
	loaded, err := LoadModuleFS(context.Background(), fsys, "root")
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.HasPackage("example.com/root/app") || loaded.HasPackage("example.com/root/nested/child") {
		t.Fatalf("nested module entered owning inventory: %+v", loaded.Packages)
	}
	fsys["root/bad/bad.go"] = &fstest.MapFile{Data: []byte("package bad\ntype {\n")}
	if _, err = LoadModuleFS(context.Background(), fsys, "root"); err == nil {
		t.Fatal("malformed package was silently omitted")
	}
}

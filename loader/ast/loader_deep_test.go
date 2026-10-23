package ast

import (
	"context"
	"testing"
	"testing/fstest"
)

func TestLoadPackageDeepFS_Cycle(t *testing.T) {
	fsys := fstest.MapFS{
		"root/go.mod": &fstest.MapFile{Data: []byte("module example.com/deep\n\n")},
		"root/a/a.go": &fstest.MapFile{Data: []byte("package a\n\nimport \"example.com/deep/b\"\n type A struct{ B *b.B }\n")},
		"root/b/b.go": &fstest.MapFile{Data: []byte("package b\n\nimport \"example.com/deep/c\"\n type B struct{ C *c.C }\n")},
		"root/c/c.go": &fstest.MapFile{Data: []byte("package c\n\nimport \"example.com/deep/a\"\n type C struct{ A *a.A }\n")},
	}
	ctx := context.Background()
	pkg, err := LoadPackageDeepFS(ctx, fsys, "root/a")
	if err != nil {
		t.Fatalf("LoadPackageDeepFS error: %v", err)
	}
	if pkg == nil || pkg.Name != "a" {
		t.Fatalf("expected root package a, got %#v", pkg)
	}
	if len(pkg.Dependencies) == 0 {
		t.Fatalf("expected dependencies for a")
	}
}

func TestLoadPackageDeepFSSkipsNestedModuleWithParentPrefix(t *testing.T) {
	fsys := fstest.MapFS{
		"root/go.mod":                {Data: []byte("module example.com/root\n")},
		"root/app/app.go":            {Data: []byte("package app\nimport _ \"example.com/root/nested/child\"\ntype App struct{}\n")},
		"root/nested/go.mod":         {Data: []byte("module example.com/root/nested\n")},
		"root/nested/child/child.go": {Data: []byte("package child\ntype Child struct{}\n")},
	}
	loaded, err := LoadPackageDeepFS(context.Background(), fsys, "root/app")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Dependencies) != 0 {
		t.Fatalf("nested module entered deep dependencies: %+v", loaded.Dependencies)
	}
}

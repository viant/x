package ast

import (
	"context"
	"testing"
	"testing/fstest"
)

func TestSelectedModuleImportClosure(t *testing.T) {
	fsys := fstest.MapFS{
		"go.mod":         {Data: []byte("module example.com/app\ngo 1.21")},
		"api/a.go":       {Data: []byte("package api\nimport \"example.com/app/model\"\ntype Input struct { Value model.Value }")},
		"model/value.go": {Data: []byte("package model\ntype Value struct { ID int }")},
		"broken/b.go":    {Data: []byte("this is invalid Go")},
		"nested/go.mod":  {Data: []byte("module example.com/nested")},
		"nested/n.go":    {Data: []byte("package nested\ntype Value string")},
	}
	module, err := LoadModulePackagesFS(context.Background(), fsys, "api")
	if err != nil {
		t.Fatal(err)
	}
	if len(module.Packages) != 2 || module.Packages["example.com/app/api"] == nil || module.Packages["example.com/app/model"] == nil {
		t.Fatalf("packages=%v", module.Packages)
	}
	for _, directory := range []string{"broken", "nested", "../escape"} {
		if _, err := LoadModulePackagesFS(context.Background(), fsys, directory); err == nil {
			t.Fatalf("accepted %s", directory)
		}
	}
}

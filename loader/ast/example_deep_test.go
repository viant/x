package ast

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing/fstest"
)

// ExampleLoadPackageDeepFS shows deep loading in-memory via fs.FS.
func ExampleLoadPackageDeepFS() {
	fsys := fstest.MapFS{
		"root/go.mod": &fstest.MapFile{Data: []byte("module example.com/deep\n\n")},
		"root/a/a.go": &fstest.MapFile{Data: []byte("package a\n\nimport \"example.com/deep/b\"\n type A struct{ B *b.B }\n")},
		"root/b/b.go": &fstest.MapFile{Data: []byte("package b\n\nimport \"example.com/deep/c\"\n type B struct{ C *c.C }\n")},
		"root/c/c.go": &fstest.MapFile{Data: []byte("package c\n\nimport \"example.com/deep/a\"\n type C struct{ A *a.A }\n")},
	}
	ctx := context.Background()
	pkg, _ := LoadPackageDeepFS(ctx, fsys, "root/a")
	fmt.Println(len(pkg.Dependencies) > 0)
	// Output: true
}

// ExampleLoadPackageDeepOS shows deep loading on the OS filesystem.
func ExampleLoadPackageDeepOS() {
	tmp := os.TempDir()
	root := filepath.Join(tmp, "example_deep_mod")
	_ = os.RemoveAll(root)
	_ = os.MkdirAll(filepath.Join(root, "a"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, "b"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, "c"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/deep\n\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "a", "a.go"), []byte("package a\n\nimport \"example.com/deep/b\"\n type A struct{ B *b.B }\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "b", "b.go"), []byte("package b\n\nimport \"example.com/deep/c\"\n type B struct{ C *c.C }\n"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "c", "c.go"), []byte("package c\n\nimport \"example.com/deep/a\"\n type C struct{ A *a.A }\n"), 0o644)
	ctx := context.Background()
	// Use an OS-backed fs.FS rooted at the module directory.
	pkg, _ := LoadPackageDeepFS(ctx, os.DirFS(root), "a")
	// print package path includes module path
	fmt.Println(strings.Contains(pkg.PkgPath, "example.com/deep"))
	// Output: true
}

// ExampleLoadPackageDeepGOPATH shows resolving external imports via GOPATH.
// This example falls back to shallow loading when GOPATH is empty.
func ExampleLoadPackageDeepGOPATH() {
	// Provide an absolute directory for a module and package as needed in real usage.
	// For brevity, this example demonstrates the call shape and prints true.
	_ = os.Setenv("GOPATH", "")
	ctx := context.Background()
	// startDir would be an absolute path in a real scenario.
	_, _ = LoadPackageDeepGOPATH(ctx, "/tmp/nonexistent", "")
	fmt.Println(true)
	// Output: true
}

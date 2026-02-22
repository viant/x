package ast

import (
	"context"
	"fmt"
	"strings"
	"testing/fstest"
)

// ExampleLoadPackageFS demonstrates loading a package from an in-memory fs.FS
// and rendering its files.
func ExampleLoadPackageFS() {
	fsys := fstest.MapFS{
		"root/go.mod":     &fstest.MapFile{Data: []byte("module example.com/demo\n\n")},
		"root/p/types.go": &fstest.MapFile{Data: []byte("package p\n\n type T struct{ ID int }\n")},
	}
	ctx := context.Background()
	pkg, _ := LoadPackageFS(ctx, fsys, "root/p")
	files, _ := pkg.RenderFiles()
	fmt.Println(strings.Contains(files["types.go"], "type T struct"))
	// Output: true
}

// ExamplePackage_RenderFilesWithMethods demonstrates appending free-function
// stubs to rendered files.
func ExamplePackage_RenderFilesWithMethods() {
	fsys := fstest.MapFS{
		"root/go.mod":    &fstest.MapFile{Data: []byte("module example.com/demo\n\n")},
		"root/p/func.go": &fstest.MapFile{Data: []byte("package p\n\n func F() int { return 1 }\n")},
	}
	ctx := context.Background()
	pkg, _ := LoadPackageFS(ctx, fsys, "root/p")
	files, _ := pkg.RenderFilesWithMethods()
	fmt.Println(strings.Contains(files["func.go"], "func F() int"))
	// Output: true
}

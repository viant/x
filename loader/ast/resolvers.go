package ast

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
)

// GOPATHResolver returns a DeepOptions.ResolveExternal-compatible function
// which resolves an import path to (os.DirFS("/"), GOPATH/src/<importPath>).
// If gopath is empty, it falls back to $GOPATH.
func GOPATHResolver(gopath string) func(ctx context.Context, importPath string) (fs.FS, string, error) {
	if gopath == "" {
		gopath = os.Getenv("GOPATH")
	}
	return func(ctx context.Context, importPath string) (fs.FS, string, error) {
		if gopath == "" {
			return nil, "", nil
		}
		return os.DirFS("/"), filepath.Join(gopath, "src", filepath.FromSlash(importPath)), nil
	}
}

//go:build !viant_afs

package ast

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/viant/x/syntetic/model"
)

// LoadPackageDeepGOPATH resolves non-stdlib imports by mapping to directories
// under GOPATH/src and recursively loading their packages using the host
// filesystem. It starts at startDir (an absolute directory).
//
// Deprecated: prefer starting from an import path using LoadExternalModule
// in combination with GOPATHResolver or the convenience wrapper
// LoadExternalModuleGOPATH when possible. This function remains for cases
// where discovery starts at a physical directory.
func LoadPackageDeepGOPATH(ctx context.Context, startDir string, gopath string) (*model.Package, error) {
	if gopath == "" {
		gopath = os.Getenv("GOPATH")
	}
	if gopath == "" {
		return LoadPackageFS(ctx, os.DirFS("/"), startDir)
	}
	// Use the fs.FS deep loader with a GOPATH external resolver.
	opts := DeepOptions{ResolveExternal: func(ctx context.Context, importPath string) (fs.FS, string, error) {
		return os.DirFS("/"), filepath.Join(gopath, "src", filepath.FromSlash(importPath)), nil
	}}
	return LoadPackageDeepFS(ctx, os.DirFS("/"), startDir, opts)
}

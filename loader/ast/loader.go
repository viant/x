//go:build viant_afs

package ast

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	afs "github.com/viant/afs"
	"github.com/viant/x/syntetic/adapter"
)

// LoadModule loads a module identified by uri using AFS when scheme is present
// or local filesystem when uri is a local path.
func LoadModule(ctx context.Context, uri string) (*model.Module, error) {
	if hasScheme(uri) {
		svc := afs.New()
		fsys := adapter.NewFS(ctx, svc, uri)
		return LoadModuleFS(ctx, fsys, ".")
	}
	abs, _ := filepath.Abs(uri)
	fsys := os.DirFS(abs)
	return LoadModuleFS(ctx, fsys, ".")
}

// LoadPackage loads a package identified by uri using AFS or local filesystem.
func LoadPackage(ctx context.Context, uri string) (*model.Package, error) {
	if hasScheme(uri) {
		svc := afs.New()
		fsys := adapter.NewFS(ctx, svc, uri)
		return LoadPackageFS(ctx, fsys, ".")
	}
	abs, _ := filepath.Abs(uri)
	fsys := os.DirFS(abs)
	return LoadPackageFS(ctx, fsys, ".")
}

func hasScheme(uri string) bool {
	return strings.Contains(uri, "://")
}

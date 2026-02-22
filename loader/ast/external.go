package ast

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/viant/x/syntetic/model"
)

// LoadExternalModule resolves importPath via resolver and deep-loads that module.
// The same resolver is used to resolve any external dependencies discovered
// while traversing the module graph.
func LoadExternalModule(ctx context.Context, importPath string, resolver func(ctx context.Context, importPath string) (fs.FS, string, error)) (*model.Package, error) {
	if resolver == nil {
		return nil, fmt.Errorf("loader: resolver is required")
	}
	fsys, dir, err := resolver(ctx, importPath)
	if err != nil {
		return nil, err
	}
	if fsys == nil || dir == "" {
		return nil, fmt.Errorf("loader: resolver returned empty fsys/dir for %s", importPath)
	}
	opts := DeepOptions{ResolveExternal: resolver}
	return LoadPackageDeepFS(ctx, fsys, dir, opts)
}

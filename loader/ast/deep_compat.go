package ast

import (
	"context"
	"os"

	"github.com/viant/x/syntetic/model"
)

// LoadPackageDeepOS is a convenience wrapper that uses the host filesystem
// to recursively load in-module dependencies starting at an absolute dir.
func LoadPackageDeepOS(ctx context.Context, dir string, opts ...DeepOptions) (*model.Package, error) {
	return LoadPackageDeepFS(ctx, os.DirFS("/"), dir, opts...)
}

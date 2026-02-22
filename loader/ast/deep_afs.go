//go:build viant_afs

package ast

import (
	"context"

	afs "github.com/viant/afs"
	"github.com/viant/x/syntetic/adapter"
	"github.com/viant/x/syntetic/model"
)

// LoadPackageDeepAFS builds an io/fs adapter for the provided AFS service and
// loads the package and its in-module dependencies from rootURI.
//
// Note: when starting from an import path rather than a root URI, prefer
// LoadExternalModule with AFSGOPATHResolver or the helper
// LoadExternalModuleAFSGOPATH to unify entrypoints.
func LoadPackageDeepAFS(ctx context.Context, svc afs.Service, rootURI string) (*model.Package, error) {
	fsys := adapter.NewFS(ctx, svc, rootURI)
	return LoadPackageDeepFS(ctx, fsys, ".")
}

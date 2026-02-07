//go:build viant_afs

package ast

import (
	"context"

	afs "github.com/viant/afs"
	"github.com/viant/x/syntetic/model"
)

// LoadExternalModuleAFSGOPATH deep-loads a module identified by importPath
// using an AFS-backed GOPATH-style resolver rooted at baseURL. The resolver
// maps to fs: baseURL + "/src/<importPath>".
//
// This is a thin helper around LoadExternalModule and AFSGOPATHResolver.
func LoadExternalModuleAFSGOPATH(ctx context.Context, importPath string, svc afs.Service, baseURL string) (*model.Package, error) {
	return LoadExternalModule(ctx, importPath, AFSGOPATHResolver(svc, baseURL))
}

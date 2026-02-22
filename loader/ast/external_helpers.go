package ast

import (
	"context"

	"github.com/viant/x/syntetic/model"
)

// LoadExternalModuleGOPATH deep-loads a module identified by importPath using a
// GOPATH-style resolver (mapping to $GOPATH/src/<importPath>). It is a thin
// convenience wrapper around LoadExternalModule and GOPATHResolver.
//
// Prefer this helper when you have an import path and want resolution based on
// a traditional GOPATH layout. For custom resolution, pass your own resolver to
// LoadExternalModule.
func LoadExternalModuleGOPATH(ctx context.Context, importPath, gopath string) (*model.Package, error) {
	return LoadExternalModule(ctx, importPath, GOPATHResolver(gopath))
}

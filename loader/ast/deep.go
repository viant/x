// Package loader: deep.go provides recursive package loading on top of fs.FS
// without relying on GOPATH semantics. It walks non-stdlib imports that are
// within the same module and attaches loaded packages to Dependencies.
package ast

import (
	"context"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/viant/x/syntetic/model"
)

// DeepOptions controls deep loading resolution for imports outside the current module.
// When ResolveExternal is provided, it is invoked for non-stdlib imports that
// are not within the current module to return an fs.FS and a directory that
// roots that import path. Returning a nil fs.FS disables resolution for that
// import (it will be skipped).
type DeepOptions struct {
	ResolveExternal func(ctx context.Context, importPath string) (fs.FS, string, error)
}

// LoadPackageDeepFS loads a package from dir and recursively loads its
// non-stdlib dependencies using fsys. It automatically follows in-module
// imports. External imports can be resolved using opts.ResolveExternal.
func LoadPackageDeepFS(ctx context.Context, fsys fs.FS, dir string, opts ...DeepOptions) (*model.Package, error) {
	moduleRoot, modulePath, err := discoverModuleFS(fsys, dir)
	if err != nil {
		return nil, err
	}
	visited := map[string]*model.Package{}
	var opt *DeepOptions
	if len(opts) > 0 {
		opt = &opts[0]
	}
	return loadPackageDeepFS(ctx, fsys, dir, moduleRoot, modulePath, visited, opt)
}

func loadPackageDeepFS(ctx context.Context, fsys fs.FS, dir, moduleRoot, modulePath string, visited map[string]*model.Package, opt *DeepOptions) (*model.Package, error) {
	pkg, err := LoadPackageFS(ctx, fsys, dir)
	if err != nil {
		return nil, err
	}
	if _, ok := visited[pkg.PkgPath]; ok {
		return pkg, nil
	}
	visited[pkg.PkgPath] = pkg

	for _, imp := range pkg.Imports {
		if isStdlibImport(imp.Path) {
			continue
		}
		// Only load dependencies within the same module
		if imp.Path == modulePath || strings.HasPrefix(imp.Path, modulePath+"/") {
			rel := strings.TrimPrefix(imp.Path, modulePath)
			rel = strings.TrimPrefix(rel, "/")
			depDir := path.Join(moduleRoot, filepath.ToSlash(rel))
			depPkg, err := loadPackageDeepFS(ctx, fsys, depDir, moduleRoot, modulePath, visited, opt)
			if err != nil {
				return nil, err
			}
			pkg.Dependencies = append(pkg.Dependencies, depPkg)
			continue
		}
		// External import: attempt resolution via options.
		if opt != nil && opt.ResolveExternal != nil {
			extFS, extDir, err := opt.ResolveExternal(ctx, imp.Path)
			if err != nil {
				return nil, err
			}
			if extFS == nil || extDir == "" {
				continue
			}
			// Discover module for the external FS/dir and recurse.
			extRoot, extMod, err := discoverModuleFS(extFS, extDir)
			if err != nil {
				return nil, err
			}
			depPkg, err := loadPackageDeepFS(ctx, extFS, extDir, extRoot, extMod, visited, opt)
			if err != nil {
				return nil, err
			}
			pkg.Dependencies = append(pkg.Dependencies, depPkg)
		}
	}
	return pkg, nil
}

// isStdlibImport makes a conservative guess whether the provided
// import path refers to a standard library package.
func isStdlibImport(p string) bool { return !strings.Contains(p, ".") }

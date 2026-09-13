package ast

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	xmodule "github.com/viant/x/module"
	"github.com/viant/x/syntetic/model"
)

// LocalPackageLoader retains selected roots separately from their local import
// closure. Callers may register private types without discovering their routes.
type LocalPackageLoader struct{ Workspace *xmodule.Workspace }
type PackageSet struct {
	Roots    []*model.Package
	Packages map[string]*model.Package
}

func (l LocalPackageLoader) Load(ctx context.Context, imports ...string) (*PackageSet, error) {
	if l.Workspace == nil {
		return nil, fmt.Errorf("local package workspace is required")
	}
	result := &PackageSet{Packages: map[string]*model.Package{}}
	active := map[string]bool{}
	var load func(string) (*model.Package, error)
	load = func(importPath string) (*model.Package, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if active[importPath] {
			return nil, fmt.Errorf("local package import cycle at %s", importPath)
		}
		if existing := result.Packages[importPath]; existing != nil {
			return existing, nil
		}
		location, err := l.Workspace.Package(importPath)
		if err != nil || location == nil {
			return nil, err
		}
		relative, err := filepath.Rel(location.Module.Dir, location.Dir)
		if err != nil {
			return nil, err
		}
		pkg, err := LoadPackageFS(ctx, os.DirFS(location.Module.Dir), filepath.ToSlash(relative))
		if err != nil {
			return nil, err
		}
		if pkg == nil || pkg.PkgPath != importPath {
			return nil, fmt.Errorf("package %s did not load with canonical import identity", importPath)
		}
		active[importPath] = true
		result.Packages[importPath] = pkg
		paths := make([]string, 0, len(pkg.Imports))
		for _, dependency := range pkg.Imports {
			paths = append(paths, dependency.Path)
		}
		sort.Strings(paths)
		seenDependencies := map[string]bool{}
		for _, path := range paths {
			if seenDependencies[path] {
				continue
			}
			seenDependencies[path] = true
			dependency, err := load(path)
			if err != nil {
				return nil, err
			}
			if dependency != nil {
				pkg.Dependencies = append(pkg.Dependencies, dependency)
			}
		}
		delete(active, importPath)
		return pkg, nil
	}
	paths := append([]string(nil), imports...)
	sort.Strings(paths)
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		pkg, err := load(path)
		if err != nil {
			return nil, err
		}
		if pkg == nil {
			return nil, fmt.Errorf("selected package %s is not available in the local workspace", path)
		}
		result.Roots = append(result.Roots, pkg)
	}
	return result, nil
}

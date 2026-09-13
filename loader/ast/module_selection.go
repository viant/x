package ast

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	xmodule "github.com/viant/x/module"
	"github.com/viant/x/syntetic/model"
)

// LoadModulePackagesFS loads selected package directories and their same-module
// import closure. Unrelated packages are never parsed; nested modules remain
// separate authorities.
func LoadModulePackagesFS(ctx context.Context, fsys fs.FS, directories ...string) (*model.Module, error) {
	info, err := xmodule.LocateFS(fsys, ".")
	if err != nil {
		return nil, err
	}
	selection := moduleSelection{fsys: fsys, info: info, module: &model.Module{Path: info.Path}, seen: map[string]bool{}}
	queue := append([]string(nil), directories...)
	sort.Strings(queue)
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		dir := path.Clean(queue[0])
		queue = queue[1:]
		if !fs.ValidPath(dir) {
			return nil, fmt.Errorf("invalid package directory %q", dir)
		}
		if selection.seen[dir] {
			continue
		}
		selection.seen[dir] = true
		dependencies, err := selection.load(ctx, dir)
		if err != nil {
			return nil, err
		}
		queue = append(queue, dependencies...)
	}
	return selection.module, nil
}

type moduleSelection struct {
	fsys   fs.FS
	info   *xmodule.Info
	module *model.Module
	seen   map[string]bool
}

func (s *moduleSelection) load(ctx context.Context, directory string) ([]string, error) {
	owner, err := xmodule.LocateFS(s.fsys, directory)
	if err != nil {
		return nil, err
	}
	if owner.Dir != s.info.Dir || owner.Path != s.info.Path {
		return nil, fmt.Errorf("package %s belongs to a nested module", directory)
	}
	pkg, err := LoadPackageFS(ctx, s.fsys, directory)
	if err != nil {
		return nil, err
	}
	if pkg == nil {
		return nil, fmt.Errorf("package %s did not load", directory)
	}
	s.module.AddPackage(pkg)
	var dependencies []string
	for _, imported := range pkg.Imports {
		if imported.Path == s.info.Path {
			dependencies = append(dependencies, s.info.Dir)
		} else if strings.HasPrefix(imported.Path, s.info.Path+"/") {
			dependencies = append(dependencies, path.Join(s.info.Dir, strings.TrimPrefix(imported.Path, s.info.Path+"/")))
		}
	}
	sort.Strings(dependencies)
	return dependencies, nil
}

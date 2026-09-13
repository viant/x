// Package loader: module.go contains module-level discovery and walking logic.
// It locates the module root (go.mod), parses the module path, and walks
// the directory tree to find packages, delegating to LoadPackageFS.
package ast

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"strings"

	xmodule "github.com/viant/x/module"
	"github.com/viant/x/syntetic/model"
)

// LoadModuleFS walks a module rooted at root within fsys, discovers the module
// path from go.mod, and loads all packages recursively.
func LoadModuleFS(ctx context.Context, fsys fs.FS, root string) (*model.Module, error) {
	if fsys == nil {
		return nil, fmt.Errorf("loader: fsys is required")
	}
	if root == "" {
		root = "."
	}
	moduleRoot, modulePath, err := discoverModuleFS(fsys, root)
	if err != nil {
		return nil, err
	}
	mod := &model.Module{Path: modulePath}
	err = fs.WalkDir(fsys, moduleRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := path.Base(p)
			if strings.HasPrefix(base, ".") || base == "vendor" {
				if p != moduleRoot {
					return fs.SkipDir
				}
			}
		} else {
			return nil
		}
		pkg, err := LoadPackageFS(ctx, fsys, p)
		if err != nil {
			return nil
		}
		if pkg != nil && len(pkg.Types) > 0 {
			mod.AddPackage(pkg)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return mod, nil
}

// discoverModuleFS finds module root and module path by walking up to go.mod.
func discoverModuleFS(fsys fs.FS, start string) (string, string, error) {
	info, err := xmodule.LocateFS(fsys, start)
	if err != nil {
		return "", "", err
	}
	return info.Dir, info.Path, nil
}

// parseModulePath extracts module path line from go.mod content.
func parseModulePath(content string) string {
	modulePath, _ := xmodule.ParsePath("go.mod", []byte(content))
	return modulePath
}

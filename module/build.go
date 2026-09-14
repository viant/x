package module

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// BuildWorkspace delegates module, workspace and target selection to the Go command.
// Env is a complete command environment; nil inherits the caller's environment.
// Patterns use Go's package syntax. No command in this owner executes application code.
type BuildWorkspace struct {
	BaseDir  string
	Patterns []string
	Tags     string
	Env      []string
	Overlay  string
}

// BuildSelection is a serializable snapshot of Go's source selection. Paths remain
// source-backed; it is not a relocatable deployment bundle.
type BuildSelection struct{ Packages []BuildPackage }
type BuildPackage struct {
	Dir, ImportPath, Name string
	GoFiles, CgoFiles     []string
	DepOnly, Standard     bool
	Module                *BuildModule
	Error                 *BuildError
	DepsErrors            []BuildError
}
type BuildModule struct {
	Path, Dir string
	Main      bool
}
type BuildError struct{ Err string }

func (b BuildWorkspace) Resolve(ctx context.Context) (*BuildSelection, error) {
	args := []string{"list", "-json", "-deps"}
	if b.Tags != "" {
		args = append(args, "-tags", b.Tags)
	}
	if b.Overlay != "" {
		args = append(args, "-overlay", b.Overlay)
	}
	patterns := b.Patterns
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	args = append(args, patterns...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = b.BaseDir
	cmd.Env = b.Env
	var out, diag bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &diag
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("select Go build: %w: %s", err, strings.TrimSpace(diag.String()))
	}
	result := &BuildSelection{}
	decoder := json.NewDecoder(&out)
	for {
		var pkg BuildPackage
		err := decoder.Decode(&pkg)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if pkg.Error != nil {
			return nil, fmt.Errorf("package %s: %s", pkg.ImportPath, pkg.Error.Err)
		}
		if len(pkg.DepsErrors) > 0 {
			return nil, fmt.Errorf("package %s: %s", pkg.ImportPath, pkg.DepsErrors[0].Err)
		}
		result.Packages = append(result.Packages, pkg)
	}
	sort.Slice(result.Packages, func(i, j int) bool { return result.Packages[i].ImportPath < result.Packages[j].ImportPath })
	return result, nil
}
func (s *BuildSelection) Workspace() *Workspace { return &Workspace{selection: s} }

func (s *BuildSelection) location(importPath string) *PackageLocation {
	for _, pkg := range s.Packages {
		if pkg.ImportPath == importPath && pkg.Module != nil {
			return &PackageLocation{Module: &Info{Path: pkg.Module.Path, Dir: pkg.Module.Dir}, Dir: pkg.Dir, ImportPath: importPath}
		}
	}
	return nil
}
func (s *BuildSelection) walk(ctx context.Context, include, exclude []string, visit func(File) error) error {
	match := LocalDiscovery{Include: include, Exclude: exclude}
	if err := match.validatePatterns(); err != nil {
		return err
	}
	if visit == nil {
		return fmt.Errorf("package visitor is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, pkg := range s.Packages {
		if pkg.DepOnly || !match.matches(include, pkg.ImportPath) || match.matches(exclude, pkg.ImportPath) {
			continue
		}
		entries, err := os.ReadDir(pkg.Dir)
		if err != nil {
			return err
		}
		allowed := map[string]bool{}
		for _, file := range append(append([]string{}, pkg.GoFiles...), pkg.CgoFiles...) {
			allowed[file] = true
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			name := entry.Name()
			if entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				continue
			}
			if strings.HasSuffix(name, ".go") && !allowed[name] {
				continue
			}
			if err := visit(File{Path: filepath.Join(pkg.Dir, name), Dir: pkg.Dir, ImportPath: pkg.ImportPath}); err != nil {
				return err
			}
		}
	}
	return nil
}

// SourceFS retains normal resource access but filters Go directory entries to
// the exact build-selected files. Metadata loaders share the compiler's choice.
func (w *Workspace) SourceFS(info *Info) fs.FS {
	source := os.DirFS(info.Dir)
	if w.selection == nil {
		return source
	}
	selected := map[string]bool{}
	for _, pkg := range w.selection.Packages {
		for _, file := range append(append([]string{}, pkg.GoFiles...), pkg.CgoFiles...) {
			selected[filepath.Join(pkg.Dir, file)] = true
		}
	}
	return &selectedFS{FS: source, root: info.Dir, selected: selected}
}

type selectedFS struct {
	fs.FS
	root     string
	selected map[string]bool
}

func (s *selectedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(s.FS, name)
	if err != nil {
		return nil, err
	}
	result := entries[:0]
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !s.selected[filepath.Join(s.root, filepath.FromSlash(name), entry.Name())] {
			continue
		}
		result = append(result, entry)
	}
	return result, nil
}

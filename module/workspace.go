package module

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	gomodule "golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

// LocalWorkspace resolves explicitly available local source modules. It never
// downloads modules or treats imported dependency packages as selected roots.
type LocalWorkspace struct {
	BaseDir    string
	ModuleDirs []string
}
type Workspace struct {
	selection    *BuildSelection
	modules      map[string]*Info
	roots        []workspaceRoot
	replacements map[string][]localReplacement
	required     map[string]string
}
type workspaceRoot struct {
	module    *Info
	directory string
}
type localReplacement struct {
	directory, version string
	work               bool
}
type PackageLocation struct {
	Module          *Info
	Dir, ImportPath string
}

func (w LocalWorkspace) Resolve(ctx context.Context) (*Workspace, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	base := w.BaseDir
	if base == "" {
		base = "."
	}
	base, err := filepath.Abs(base)
	if err != nil {
		return nil, err
	}
	result := &Workspace{modules: map[string]*Info{}, replacements: map[string][]localReplacement{}, required: map[string]string{}}
	roots := append([]string(nil), w.ModuleDirs...)
	if name, err := enclosingFile(base, "go.mod"); err != nil {
		return nil, err
	} else if name != "" {
		roots = append(roots, base)
	}
	var work *modfile.WorkFile
	var workDir string
	if name, err := enclosingFile(base, "go.work"); err != nil {
		return nil, err
	} else if name != "" {
		content, err := os.ReadFile(name)
		if err != nil {
			return nil, err
		}
		work, err = modfile.ParseWork(name, content, nil)
		if err != nil {
			return nil, err
		}
		workDir = filepath.Dir(name)
		for _, use := range work.Use {
			directory := use.Path
			if !filepath.IsAbs(directory) {
				directory = filepath.Join(workDir, directory)
			}
			roots = append(roots, directory)
		}
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("no local modules selected from %s", base)
	}
	for _, directory := range roots {
		if !filepath.IsAbs(directory) {
			directory = filepath.Join(base, directory)
		}
		directory, err = filepath.EvalSymlinks(directory)
		if err != nil {
			return nil, err
		}
		info, err := LocateLocal(directory)
		if err != nil {
			return nil, err
		}
		if previous := result.modules[info.Path]; previous != nil && previous.Dir != info.Dir {
			paths := []string{previous.Dir, info.Dir}
			sort.Strings(paths)
			return nil, fmt.Errorf("module %s is present in both %s and %s", info.Path, paths[0], paths[1])
		}
		result.modules[info.Path] = info
		duplicate := false
		for _, root := range result.roots {
			if root.directory == directory {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result.roots = append(result.roots, workspaceRoot{module: info, directory: directory})
		}
	}
	sort.Slice(result.roots, func(i, j int) bool {
		if result.roots[i].module.Path != result.roots[j].module.Path {
			return result.roots[i].module.Path < result.roots[j].module.Path
		}
		return result.roots[i].directory < result.roots[j].directory
	})
	paths := make([]string, 0, len(result.modules))
	for name := range result.modules {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	// Only main/root manifests supply replacements. Dependency-module replace
	// directives are deliberately not inherited, matching Go module authority.
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		info := result.modules[path]
		name := filepath.Join(info.Dir, "go.mod")
		content, err := os.ReadFile(name)
		if err != nil {
			return nil, err
		}
		file, err := modfile.Parse(name, content, nil)
		if err != nil {
			return nil, err
		}
		for _, required := range file.Require {
			if previous := result.required[required.Mod.Path]; previous == "" || semver.Compare(required.Mod.Version, previous) > 0 {
				result.required[required.Mod.Path] = required.Mod.Version
			}
		}
		for _, replacement := range file.Replace {
			result.addReplacement(info.Dir, replacement, false)
		}
	}
	if work != nil {
		for _, replacement := range work.Replace {
			result.addReplacement(workDir, replacement, true)
		}
	}
	return result, nil
}

func (w *Workspace) addReplacement(base string, replacement *modfile.Replace, work bool) {
	directory := ""
	if replacement.New.Version == "" {
		directory = replacement.New.Path
		if !filepath.IsAbs(directory) {
			directory = filepath.Join(base, directory)
		}
		directory = filepath.Clean(directory)
	}
	w.replacements[replacement.Old.Path] = append(w.replacements[replacement.Old.Path], localReplacement{directory: directory, version: replacement.Old.Version, work: work})
}

func (w *Workspace) replacement(modulePath string) (string, error) {
	entries := w.replacements[modulePath]
	required := w.required[modulePath]
	choose := func(work bool) []localReplacement {
		var exact, wildcard []localReplacement
		for _, entry := range entries {
			if entry.work != work {
				continue
			}
			if entry.version == "" {
				wildcard = append(wildcard, entry)
			} else if required != "" && entry.version == required {
				exact = append(exact, entry)
			}
		}
		if len(exact) > 0 {
			return exact
		}
		return wildcard
	}
	candidates := choose(true)
	if len(candidates) == 0 {
		candidates = choose(false)
	}
	if len(candidates) == 0 {
		if required == "" && len(entries) > 0 {
			return "", fmt.Errorf("version-specific replacement for %s requires an explicit main-module requirement", modulePath)
		}
		return "", nil
	}
	directory := candidates[0].directory
	for _, candidate := range candidates[1:] {
		if candidate.directory != directory {
			paths := []string{directory, candidate.directory}
			sort.Strings(paths)
			return "", fmt.Errorf("conflicting local replacements for %s: %s and %s", modulePath, paths[0], paths[1])
		}
	}
	return directory, nil
}

// Package resolves roots and applicable local replacements lazily. Missing
// unused developer replacements cannot invalidate unrelated selected packages.
func (w *Workspace) Package(importPath string) (*PackageLocation, error) {
	if w != nil && w.selection != nil {
		return w.selection.location(importPath), nil
	}
	if w == nil {
		return nil, fmt.Errorf("local workspace is required")
	}
	if err := gomodule.CheckImportPath(importPath); err != nil {
		return nil, err
	}
	prefix := ""
	match := func(candidate string) {
		if importPath == candidate || strings.HasPrefix(importPath, candidate+"/") {
			if len(candidate) > len(prefix) {
				prefix = candidate
			}
		}
	}
	for path := range w.modules {
		match(path)
	}
	for path := range w.replacements {
		match(path)
	}
	if prefix == "" {
		return nil, nil
	}
	selected := w.modules[prefix]
	if selected == nil {
		directory, err := w.replacement(prefix)
		if err != nil {
			return nil, err
		}
		if directory == "" {
			return nil, nil
		}
		directory, err = filepath.EvalSymlinks(directory)
		if err != nil {
			return nil, fmt.Errorf("local replacement %s: %w", prefix, err)
		}
		selected, err = LocateLocal(directory)
		if err != nil {
			return nil, err
		}
		if selected.Path != prefix {
			return nil, fmt.Errorf("local replacement %s declares module %s", prefix, selected.Path)
		}
	}
	relative := strings.TrimPrefix(strings.TrimPrefix(importPath, selected.Path), "/")
	directory := filepath.Join(selected.Dir, filepath.FromSlash(relative))
	actual, err := ImportPathLocal(selected.Dir, selected.Path, directory)
	if err != nil || actual != importPath {
		return nil, fmt.Errorf("invalid local import path %q", importPath)
	}
	owner, err := LocateLocal(directory)
	if err != nil {
		return nil, err
	}
	if owner.Path != selected.Path || owner.Dir != selected.Dir {
		return nil, fmt.Errorf("package %s crosses nested module authority", importPath)
	}
	stat, err := os.Stat(directory)
	if err != nil {
		return nil, fmt.Errorf("local package %s: %w", importPath, err)
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("local package %s is not a directory", importPath)
	}
	copy := *selected
	return &PackageLocation{Module: &copy, Dir: directory, ImportPath: importPath}, nil
}

func (w *Workspace) Walk(ctx context.Context, include, exclude []string, visit func(File) error) error {
	if w != nil && w.selection != nil {
		return w.selection.walk(ctx, include, exclude, visit)
	}
	if w == nil || len(w.roots) == 0 {
		return fmt.Errorf("local workspace is required")
	}
	if visit == nil {
		return fmt.Errorf("package visitor is required")
	}
	seen := map[string]bool{}
	for _, root := range w.roots {
		if err := (LocalDiscovery{BaseDir: root.directory, Include: include, Exclude: exclude}).Walk(ctx, func(file File) error {
			if seen[file.Path] {
				return nil
			}
			seen[file.Path] = true
			return visit(file)
		}); err != nil {
			return err
		}
	}
	return nil
}

func enclosingFile(start, name string) (string, error) {
	directory, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(directory, name)
		_, err := os.Stat(candidate)
		if err == nil {
			return candidate, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", nil
		}
		directory = parent
	}
}

// Package module owns Go module identity and source discovery.
package module

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

type Info struct {
	Dir  string
	Path string
}

// ParsePath reads the module directive using Go's module-file parser.
func ParsePath(name string, content []byte) (string, error) {
	file, err := modfile.ParseLax(name, content, nil)
	if err != nil {
		return "", err
	}
	if file.Module == nil || strings.TrimSpace(file.Module.Mod.Path) == "" {
		return "", fmt.Errorf("%s: module directive is required", name)
	}
	return file.Module.Mod.Path, nil
}

// LocateLocal accepts an existing file/directory or a not-yet-created package
// directory and finds its nearest enclosing module without changing disk state.
func LocateLocal(start string) (*Info, error) {
	if strings.TrimSpace(start) == "" {
		return nil, fmt.Errorf("module location is required")
	}
	current, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	if info, statErr := os.Stat(current); statErr == nil && !info.IsDir() {
		current = filepath.Dir(current)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return nil, statErr
	}
	for {
		name := filepath.Join(current, "go.mod")
		content, readErr := os.ReadFile(name)
		if readErr == nil {
			modulePath, err := ParsePath(name, content)
			if err != nil {
				return nil, err
			}
			return &Info{Dir: current, Path: modulePath}, nil
		}
		if !os.IsNotExist(readErr) {
			return nil, readErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil, fmt.Errorf("go.mod not found from %s", start)
		}
		current = parent
	}
}

func LocateFS(fsys fs.FS, start string) (*Info, error) {
	if fsys == nil {
		return nil, fmt.Errorf("module filesystem is required")
	}
	if start == "" {
		start = "."
	}
	current := path.Clean(start)
	if !fs.ValidPath(current) {
		return nil, fmt.Errorf("invalid module path %q", start)
	}
	if info, err := fs.Stat(fsys, current); err == nil && !info.IsDir() {
		current = path.Dir(current)
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for {
		name := path.Join(current, "go.mod")
		content, err := fs.ReadFile(fsys, name)
		if err == nil {
			modulePath, err := ParsePath(name, content)
			if err != nil {
				return nil, err
			}
			return &Info{Dir: current, Path: modulePath}, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
		if current == "." {
			return nil, fmt.Errorf("go.mod not found from %s", start)
		}
		current = path.Dir(current)
	}
}

func ImportPathLocal(root, modulePath, directory string) (string, error) {
	if strings.TrimSpace(modulePath) == "" {
		return "", fmt.Errorf("module import path is required")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	directory, err = filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, directory)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("package directory %s escapes module %s", directory, root)
	}
	if relative == "." {
		return modulePath, nil
	}
	return strings.TrimSuffix(modulePath, "/") + "/" + filepath.ToSlash(relative), nil
}

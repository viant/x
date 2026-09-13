package module

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type File struct {
	Path       string
	Dir        string
	ImportPath string
}

type LocalDiscovery struct {
	BaseDir string
	Include []string
	Exclude []string
}

func (d LocalDiscovery) Walk(ctx context.Context, visit func(File) error) error {
	if len(d.Include) == 0 {
		return fmt.Errorf("package include patterns are required")
	}
	if visit == nil {
		return fmt.Errorf("package visitor is required")
	}
	for _, pattern := range append(append([]string(nil), d.Include...), d.Exclude...) {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("empty package pattern")
		}
		if _, err := path.Match(strings.TrimSuffix(pattern, "/..."), ""); err != nil {
			return fmt.Errorf("invalid package pattern %q: %w", pattern, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := LocateLocal(d.BaseDir)
	if err != nil {
		return err
	}
	base, err := filepath.Abs(d.BaseDir)
	if err != nil {
		return err
	}
	stat, err := os.Stat(base)
	if err != nil {
		return err
	}
	if !stat.IsDir() {
		return fmt.Errorf("package discovery base must be a directory")
	}
	return filepath.WalkDir(base, func(location string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		name := entry.Name()
		if entry.IsDir() {
			if location != base && (name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			if location != info.Dir {
				if _, err := os.Stat(filepath.Join(location, "go.mod")); err == nil {
					return filepath.SkipDir
				} else if !os.IsNotExist(err) {
					return err
				}
			}
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			return nil
		}
		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if !entryInfo.Mode().IsRegular() {
			return nil
		}
		directory := filepath.Dir(location)
		importPath, err := ImportPathLocal(info.Dir, info.Path, directory)
		if err != nil {
			return err
		}
		if !d.matches(d.Include, importPath) || d.matches(d.Exclude, importPath) {
			return nil
		}
		return visit(File{Path: location, Dir: directory, ImportPath: importPath})
	})
}

func (d LocalDiscovery) matches(patterns []string, importPath string) bool {
	for _, pattern := range patterns {
		if pattern == "..." {
			return true
		}
		if strings.HasSuffix(pattern, "/...") {
			prefix := strings.TrimSuffix(pattern, "/...")
			if importPath == prefix || strings.HasPrefix(importPath, prefix+"/") {
				return true
			}
			continue
		}
		if matched, _ := path.Match(pattern, importPath); matched {
			return true
		}
	}
	return false
}

package module

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const goIgnoreName = ".goignore"

type goIgnore struct {
	paths []string
}

func readGoIgnore(root string) (*goIgnore, error) {
	name := filepath.Join(root, goIgnoreName)
	data, err := os.ReadFile(name)
	if os.IsNotExist(err) {
		return &goIgnore{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("%s: expected UTF-8 text", name)
	}
	result := &goIgnore{}
	for index, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		portable := filepath.ToSlash(line)
		if filepath.IsAbs(line) || path.IsAbs(portable) {
			return nil, fmt.Errorf("%s:%d: path must be module-relative", name, index+1)
		}
		clean := path.Clean(portable)
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, fmt.Errorf("%s:%d: path escapes module root", name, index+1)
		}
		result.paths = append(result.paths, clean)
	}
	return result, nil
}

func (g *goIgnore) matches(relative string) bool {
	if g == nil {
		return false
	}
	for _, ignored := range g.paths {
		if relative == ignored || strings.HasPrefix(relative, ignored+"/") {
			return true
		}
	}
	return false
}

package model

import (
	"go/ast"
	"path"
	"sort"
)

// ImportSet tracks import paths and their assigned aliases in a deterministic
// way. It is used by AST generation to qualify identifiers and later by the
// syntetic bridge when emitting import specs.
type ImportSet struct {
	CurrentPkg string

	byPath  map[string]string // path -> alias
	byAlias map[string]string // alias -> path
}

// NewImportSet creates a new ImportSet for the provided current package path.
func NewImportSet(currentPkg string) *ImportSet {
	return &ImportSet{
		CurrentPkg: currentPkg,
		byPath:     map[string]string{},
		byAlias:    map[string]string{},
	}
}

// AliasFor returns a deterministic alias for the supplied import path.
// The result is cached for subsequent calls.
func (s *ImportSet) AliasFor(pkgPath string) string {
	if pkgPath == "" || pkgPath == s.CurrentPkg {
		return ""
	}
	if s.byPath == nil {
		s.byPath = map[string]string{}
	}
	if s.byAlias == nil {
		s.byAlias = map[string]string{}
	}
	if alias, ok := s.byPath[pkgPath]; ok {
		return alias
	}
	base := path.Base(pkgPath)
	alias := sanitizeAlias(base)
	if alias == "" {
		alias = "pkg"
	}
	original := alias
	for i := 0; ; i++ {
		if i > 0 {
			alias = original + itoa(i)
		}
		if _, exists := s.byAlias[alias]; !exists {
			break
		}
	}
	s.byPath[pkgPath] = alias
	s.byAlias[alias] = pkgPath
	return alias
}

// QualIdent returns an AST expression representing ident qualified with the
// import alias for pkgPath when needed.
func (s *ImportSet) QualIdent(pkgPath, ident string) ast.Expr {
	if pkgPath == "" || pkgPath == s.CurrentPkg {
		return ast.NewIdent(ident)
	}
	alias := s.AliasFor(pkgPath)
	if alias == "" {
		return ast.NewIdent(ident)
	}
	return &ast.SelectorExpr{X: ast.NewIdent(alias), Sel: ast.NewIdent(ident)}
}

// ImportEntry represents a single import path and alias.
type ImportEntry struct {
	Alias string
	Path  string
}

// Entries returns a deterministically sorted slice of imports.
func (s *ImportSet) Entries() []ImportEntry {
	if s == nil || len(s.byPath) == 0 {
		return nil
	}
	entries := make([]ImportEntry, 0, len(s.byPath))
	for p, a := range s.byPath {
		entries = append(entries, ImportEntry{Alias: a, Path: p})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Alias == entries[j].Alias {
			return entries[i].Path < entries[j].Path
		}
		return entries[i].Alias < entries[j].Alias
	})
	return entries
}

// sanitizeAlias converts an arbitrary path segment into a valid Go identifier
// suitable for use as an import alias.
func sanitizeAlias(segment string) string {
	if segment == "" {
		return ""
	}
	runes := []rune(segment)
	for i, r := range runes {
		if !(r == '_' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			runes[i] = '_'
		}
	}
	if first := runes[0]; !(first == '_' || (first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z')) {
		runes = append([]rune{'_'}, runes...)
	}
	return string(runes)
}

// itoa is a minimal integer to string converter for small non-negative values.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	buf := [10]byte{}
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}

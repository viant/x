package xtype

import (
	"go/ast"
	"path"
	"sort"
	"unicode"
)

type ImportSet struct {
	CurrentPkg string
	byPath     map[string]string
	byAlias    map[string]string
}

func NewImportSet(currentPkg string) *ImportSet {
	return &ImportSet{CurrentPkg: currentPkg, byPath: map[string]string{}, byAlias: map[string]string{}}
}

func (s *ImportSet) AliasFor(pkgPath string) string {
	if pkgPath == "" || pkgPath == s.CurrentPkg {
		return ""
	}
	if a, ok := s.byPath[pkgPath]; ok {
		return a
	}
	base := path.Base(pkgPath)
	alias := sanitizeAlias(base)
	if alias == "" {
		alias = "pkg"
	}
	if alias == "_" || alias == "." {
		alias = base + "_"
	}
	orig := alias
	for i := 0; ; i++ {
		if i > 0 {
			alias = orig + string('0'+rune(i))
		}
		if _, exists := s.byAlias[alias]; !exists {
			break
		}
	}
	s.byPath[pkgPath] = alias
	s.byAlias[alias] = pkgPath
	return alias
}

func (s *ImportSet) Add(pkgPath string) string { return s.AliasFor(pkgPath) }

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

type ImportEntry struct{ Alias, Path string }

func (s *ImportSet) Entries() []ImportEntry {
	out := make([]ImportEntry, 0, len(s.byPath))
	for p, a := range s.byPath {
		out = append(out, ImportEntry{Alias: a, Path: p})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Alias == out[j].Alias {
			return out[i].Path < out[j].Path
		}
		return out[i].Alias < out[j].Alias
	})
	return out
}

func sanitizeAlias(seg string) string {
	if seg == "" {
		return ""
	}
	r := []rune(seg)
	for i, c := range r {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' {
			r[i] = '_'
		}
	}
	if !unicode.IsLetter(r[0]) && r[0] != '_' {
		r = append([]rune{'_'}, r...)
	}
	return string(r)
}

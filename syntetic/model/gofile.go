package model

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"go/printer"
	"go/token"
	"path"
	"sort"
)

// GoFile represents a single Go source file composed of types and the
// imports needed to render them under stable, per-file aliasing.
type GoFile struct {
	// Name is an optional logical filename (e.g., types_gen.go).
	Name    string
	PkgName string
	Imports map[string]ImportRef // key: path+" "+alias
	Types   []*Type
	Consts  []ConstDecl
	Vars    []VarDecl
	// Embeds maps variable name -> embedded FS instance for variables
	// declared with //go:embed directives in this file.
	Embeds map[string]*embed.FS
}

// RenderOptions controls rendering behaviour.
type RenderOptions struct {
	// InterleaveMethodStubs places method stubs immediately after each
	// corresponding type declaration instead of batching at the end.
	InterleaveMethodStubs bool
	// ImportAliases optionally overrides the alias used when referring to
	// external packages by import path. Keys are full import paths, values
	// are aliases to use (e.g., "github.com/acme/foo" -> "fooalias"). When
	// provided, both import declarations and selector emission from model Nodes
	// will prefer these aliases.
	ImportAliases map[string]string
}

// AddEmbed registers an embedded FS instance under the provided variable name.
func (f *GoFile) AddEmbed(varName string, fsinst *embed.FS) {
	if f == nil || varName == "" || fsinst == nil {
		return
	}
	if f.Embeds == nil {
		f.Embeds = map[string]*embed.FS{}
	}
	f.Embeds[varName] = fsinst
}

// HasEmbed reports whether an embedded FS instance exists for varName.
func (f *GoFile) HasEmbed(varName string) bool {
	if f == nil || varName == "" || len(f.Embeds) == 0 {
		return false
	}
	_, ok := f.Embeds[varName]
	return ok
}

// AddType appends t to the file and merges its imports.
func (f *GoFile) AddType(t *Type) {
	if t == nil {
		return
	}
	f.Types = append(f.Types, t)
	if f.Imports == nil {
		f.Imports = map[string]ImportRef{}
	}
	for _, ref := range t.Imports {
		if ref == nil {
			continue
		}
		key := ref.Path + " " + ref.Alias
		if _, ok := f.Imports[key]; !ok {
			f.Imports[key] = *ref
		}
	}
}

// AddImport inserts an import reference if it does not already exist.
func (f *GoFile) AddImport(ref ImportRef) {
	if f.Imports == nil {
		f.Imports = map[string]ImportRef{}
	}
	key := ref.Key()
	if _, ok := f.Imports[key]; !ok {
		f.Imports[key] = ref
	}
}

// AddSideEffectImport adds an import with alias '_' for side effects.
func (f *GoFile) AddSideEffectImport(path string) {
	if path == "" {
		return
	}
	f.AddImport(ImportRef{Path: path, Alias: "_"})
}

// HasImport reports if the file has an import with the provided path/alias.
func (f *GoFile) HasImport(path, alias string) bool {
	if f == nil || len(f.Imports) == 0 {
		return false
	}
	_, ok := f.Imports[path+" "+alias]
	return ok
}

// HasType reports whether a type with the given name is queued for rendering.
func (f *GoFile) HasType(name string) bool {
	if f == nil || name == "" {
		return false
	}
	for _, t := range f.Types {
		if t != nil && t.Name == name {
			return true
		}
	}
	return false
}

// AddConst appends a const declaration and merges its imports.
func (f *GoFile) AddConst(c ConstDecl) {
	f.Consts = append(f.Consts, c)
	for _, ref := range c.Imports {
		f.AddImport(ref)
	}
}

// HasConst reports whether a const with the given name is present.
func (f *GoFile) HasConst(name string) bool {
	if f == nil || name == "" {
		return false
	}
	for _, c := range f.Consts {
		if c.Name == name {
			return true
		}
	}
	return false
}

// AddVar appends a var declaration and merges its imports.
func (f *GoFile) AddVar(v VarDecl) {
	f.Vars = append(f.Vars, v)
	for _, ref := range v.Imports {
		f.AddImport(ref)
	}
}

// HasVar reports whether a var with the given name is present.
func (f *GoFile) HasVar(name string) bool {
	if f == nil || name == "" {
		return false
	}
	for _, v := range f.Vars {
		if v.Name == name {
			return true
		}
	}
	return false
}

// Render emits a formatted Go source file for the accumulated types and imports.
func (f *GoFile) Render() (string, error) {
	return f.RenderWithOptions(RenderOptions{})
}

// RenderWithOptions renders the file with configurable behaviour.
func (f *GoFile) RenderWithOptions(opts RenderOptions) (string, error) {
	if f.PkgName == "" {
		return "", fmt.Errorf("syntetic: package name is required")
	}
	var buf bytes.Buffer
	buf.WriteString("package ")
	buf.WriteString(f.PkgName)
	buf.WriteString("\n\n")

	// Aliases to use when emitting selector expressions.
	emissionAliases := opts.ImportAliases

	if len(f.Imports) > 0 {
		// Build an effective, de-duplicated import list by path, honoring
		// optional alias overrides from opts.ImportAliases.
		effective := map[string]string{} // path -> alias (may be empty)
		aliasesPerPath := map[string]map[string]struct{}{}
		// First, apply existing imports, preferring explicit aliases.
		for _, ref := range f.Imports {
			if _, ok := effective[ref.Path]; !ok {
				effective[ref.Path] = ref.Alias
			} else {
				// If there's already an alias but this one is explicit and the
				// existing is empty, prefer the explicit one for determinism.
				if effective[ref.Path] == "" && ref.Alias != "" {
					effective[ref.Path] = ref.Alias
				}
			}
			set := aliasesPerPath[ref.Path]
			if set == nil {
				set = map[string]struct{}{}
				aliasesPerPath[ref.Path] = set
			}
			set[ref.Alias] = struct{}{}
		}
		// Then, override with provided alias map when present.
		if len(opts.ImportAliases) > 0 {
			for p, a := range opts.ImportAliases {
				if _, exists := effective[p]; exists {
					// Only override if no conflicting explicit alias exists
					// in this file (to avoid breaking pre-parsed stubs that
					// reference that alias).
					conflict := false
					if set := aliasesPerPath[p]; set != nil {
						implicit := path.Base(p)
						for existing := range set {
							// Treat empty alias as the implicit base segment.
							used := existing
							if used == "" {
								used = implicit
							}
							if used != a {
								conflict = true
								break
							}
						}
					}
					if !conflict {
						effective[p] = a
					}
				}
			}
		}
		// Emit in sorted path order for stability.
		paths := make([]string, 0, len(effective))
		for p := range effective {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		buf.WriteString("import (\n")
		for _, p := range paths {
			alias := effective[p]
			buf.WriteString("\t")
			if alias != "" {
				fmt.Fprintf(&buf, "%s \"%s\"\n", alias, p)
			} else {
				fmt.Fprintf(&buf, "\"%s\"\n", p)
			}
		}
		buf.WriteString(")\n\n")
		// Use the effective aliases for subsequent emission.
		emissionAliases = effective
	}

	// Render consts
	if len(f.Consts) > 0 {
		names := make([]string, len(f.Consts))
		idx := map[string]int{}
		for i, c := range f.Consts {
			names[i] = c.Name
			idx[c.Name] = i
		}
		sort.Strings(names)
		buf.WriteString("const (\n")
		for _, n := range names {
			c := f.Consts[idx[n]]
			buf.WriteString("\t")
			buf.WriteString(c.Name)
			typeStr := renderExpr(c.Type)
			valStr := renderExpr(c.Value)
			if typeStr != "" {
				buf.WriteString(" ")
				buf.WriteString(typeStr)
			}
			if valStr != "" {
				buf.WriteString(" = ")
				buf.WriteString(valStr)
			}
			buf.WriteString("\n")
		}
		buf.WriteString(")\n\n")
	}

	// Render vars
	if len(f.Vars) > 0 {
		names := make([]string, len(f.Vars))
		idx := map[string]int{}
		for i, v := range f.Vars {
			names[i] = v.Name
			idx[v.Name] = i
		}
		sort.Strings(names)
		buf.WriteString("var (\n")
		for _, n := range names {
			v := f.Vars[idx[n]]
			buf.WriteString("\t")
			buf.WriteString(v.Name)
			typeStr := renderExpr(v.Type)
			valStr := renderExpr(v.Value)
			if typeStr != "" {
				buf.WriteString(" ")
				buf.WriteString(typeStr)
			}
			if valStr != "" {
				buf.WriteString(" = ")
				buf.WriteString(valStr)
			}
			buf.WriteString("\n")
		}
		buf.WriteString(")\n\n")
	}

	// Render types
	if len(f.Types) > 0 {
		names := make([]string, 0, len(f.Types))
		byName := map[string]*Type{}
		for _, t := range f.Types {
			if t != nil && t.Name != "" {
				names = append(names, t.Name)
				byName[t.Name] = t
			}
		}
		sort.Strings(names)
		for i, n := range names {
			if i > 0 {
				buf.WriteString("\n")
			}
			t := byName[n]
			if t == nil {
				continue
			}
			decl := t.ToGenDecl(t.PkgPath, emissionAliases)
			if decl == nil {
				continue
			}
			_ = printer.Fprint(&buf, token.NewFileSet(), decl)
			buf.WriteString("\n")
			if opts.InterleaveMethodStubs {
				// Interleave: append stubs for this type now. This aids
				// readability by keeping methods close to their type.
				for _, s := range t.MethodStubs() {
					if s == "" {
						continue
					}
					buf.WriteString(s)
					if s[len(s)-1] != '\n' {
						buf.WriteString("\n")
					}
				}
			}
		}
		if !opts.InterleaveMethodStubs {
			// Batch: append all stubs after type declarations to avoid
			// interrupting type blocks.
			wroteStub := false
			for _, n := range names {
				t := byName[n]
				if t == nil {
					continue
				}
				for _, s := range t.MethodStubs() {
					if s == "" {
						continue
					}
					if !wroteStub {
						buf.WriteString("\n")
						wroteStub = true
					}
					buf.WriteString(s)
					if s[len(s)-1] != '\n' {
						buf.WriteString("\n")
					}
				}
			}
		}
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return "", err
	}
	return string(formatted), nil
}

// RenderWithMethods is a compatibility helper that currently renders the file
// including methods; it forwards to Render since Render already includes
// method stubs after type declarations.
func (f *GoFile) RenderWithMethods() (string, error) { return f.Render() }

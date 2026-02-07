package model

import (
	"bytes"
	"fmt"
	"go/format"
	"sort"
)

// Namespace groups a set of synthetic types along with the aggregated
// imports required to render them into a single Go source file.
type Namespace struct {
	// PkgName is the default package name used when rendering if not
	// explicitly provided.
	PkgName string
	// PkgPath is the default package import path for aliasing decisions.
	PkgPath string
	// Types holds named synthetic types in this namespace.
	Types map[string]*Type

	// TypesByPkg groups types by their package import path, then by
	// type name. This enables multi-package composition and per-package
	// file rendering.
	TypesByPkg map[string]map[string]*Type

	// Imports aggregates all unique imports required by Types. The key
	// is path + " " + alias which keeps the de-duplication logic
	// consistent with the loader's behaviour.
	Imports map[string]ImportRef

	// External optionally tracks referenced external package types for
	// documentation or higher-level tooling. The key is an import path and
	// the value is a list of type names referenced from that package.
	External map[string][]string

	// Index provides a qualified lookup keyed by "<pkgPath>.<name>" (or
	// just name when pkgPath is empty). This avoids collisions across
	// packages and provides a stable identity for metadata overlays.
	Index map[string]*Type
}

// AddType registers t in the namespace and merges its imports into the
// namespace-level Imports collection. Imports are de-duplicated by the
// composite key path + " " + alias.
func (n *Namespace) AddType(t *Type) {
	if t == nil {
		return
	}
	if n.Types == nil {
		n.Types = map[string]*Type{}
	}
	if n.TypesByPkg == nil {
		n.TypesByPkg = map[string]map[string]*Type{}
	}
	if n.Imports == nil {
		n.Imports = map[string]ImportRef{}
	}

	if t.Name != "" {
		n.Types[t.Name] = t
		// Qualified index
		key := n.KeyOf(t)
		if n.Index == nil {
			n.Index = map[string]*Type{}
		}
		n.Index[key] = t
		// Per-package grouping
		byName := n.TypesByPkg[t.PkgPath]
		if byName == nil {
			byName = map[string]*Type{}
			n.TypesByPkg[t.PkgPath] = byName
		}
		byName[t.Name] = t
	}

	for _, ref := range t.Imports {
		if ref == nil {
			continue
		}
		key := ref.Path + " " + ref.Alias
		if _, exists := n.Imports[key]; !exists {
			n.Imports[key] = *ref
		}
	}
}

// KeyOf returns a qualified key for the provided type in the form
// "<pkgPath>.<name>" when a package path is present, otherwise just the
// type name. It is safe to call with a nil receiver or argument.
func (n *Namespace) KeyOf(t *Type) string {
	if t == nil {
		return ""
	}
	if t.PkgPath == "" {
		return t.Name
	}
	return t.PkgPath + "." + t.Name
}

// AddImport adds an import reference to the namespace if it does not already
// exist. This can be useful when external types are referenced only in method
// or function stubs attached later.
func (n *Namespace) AddImport(ref ImportRef) {
	if n == nil {
		return
	}
	if n.Imports == nil {
		n.Imports = map[string]ImportRef{}
	}
	key := ref.Path + " " + ref.Alias
	if _, ok := n.Imports[key]; !ok {
		n.Imports[key] = ref
	}
}

// AddExternal records usage of a type from another package under pkgPath and
// merges a default import for that package (if not already present). Alias is
// optional; pass empty to use the package's default name.
func (n *Namespace) AddExternal(pkgPath, typeName, alias string) {
	if n == nil || pkgPath == "" || typeName == "" {
		return
	}
	if n.External == nil {
		n.External = map[string][]string{}
	}
	list := n.External[pkgPath]
	// naive de-duplication
	exists := false
	for _, v := range list {
		if v == typeName {
			exists = true
			break
		}
	}
	if !exists {
		n.External[pkgPath] = append(list, typeName)
	}
	n.AddImport(ImportRef{Path: pkgPath, Alias: alias})
}

// HasType reports whether a named type exists in the namespace.
func (n *Namespace) HasType(name string) bool {
	if n == nil || name == "" || len(n.Types) == 0 {
		return false
	}
	_, ok := n.Types[name]
	return ok
}

// HasImport reports whether an import with the given path/alias exists.
func (n *Namespace) HasImport(path, alias string) bool {
	if n == nil || len(n.Imports) == 0 {
		return false
	}
	_, ok := n.Imports[path+" "+alias]
	return ok
}

// RenderFile renders all types stored in the namespace into a single
// Go source file. It emits the package clause using pkgName, followed
// by an aggregated import block and the type declarations. The result
// is go/format formatted.
func (n *Namespace) RenderFile(pkgName string) (string, error) {
	if pkgName == "" {
		pkgName = n.PkgName
	}
	if pkgName == "" {
		return "", fmt.Errorf("syntetic: package name is required")
	}
	var buf bytes.Buffer
	buf.WriteString("package ")
	buf.WriteString(pkgName)
	buf.WriteString("\n\n")

	if len(n.Imports) > 0 {
		paths := make([]string, 0, len(n.Imports))
		ordered := map[string]ImportRef{}
		for key, ref := range n.Imports {
			paths = append(paths, key)
			ordered[key] = ref
		}
		sort.Strings(paths)
		buf.WriteString("import (\n")
		for _, key := range paths {
			ref := ordered[key]
			buf.WriteString("\t")
			if ref.Alias != "" {
				fmt.Fprintf(&buf, "%s \"%s\"\n", ref.Alias, ref.Path)
			} else {
				fmt.Fprintf(&buf, "\"%s\"\n", ref.Path)
			}
		}
		buf.WriteString(")\n\n")
	}

	if len(n.Types) > 0 {
		keys := make([]string, 0, len(n.Types))
		for name := range n.Types {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		for i, name := range keys {
			if i > 0 {
				buf.WriteString("\n")
			}
			t := n.Types[name]
			if t == nil {
				continue
			}
			body := t.Body()
			if body == "" {
				continue
			}
			fmt.Fprintf(&buf, "type %s %s\n", t.Name, body)
		}
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return "", err
	}
	return string(formatted), nil
}

// Render renders using the namespace's default package name. It is a thin
// wrapper over RenderFile.
func (n *Namespace) Render() (string, error) {
	return n.RenderFile("")
}

// BuildFiles constructs a GoFile per package import path containing the
// types that belong to that package. The returned map is keyed by pkgPath.
// Callers can render each file with Render or RenderWithOptions.
//
// Note: opts currently affects only rendering; it is accepted for future
// extension and symmetry with GoFile rendering helpers.
func (n *Namespace) BuildFiles(opts RenderOptions) (map[string]*GoFile, error) {
	files := map[string]*GoFile{}
	if n == nil || len(n.TypesByPkg) == 0 {
		return files, nil
	}
	for pkgPath, byName := range n.TypesByPkg {
		// Choose a package name. Prefer namespace's PkgName when matching
		// PkgPath, otherwise derive from the last segment of pkgPath.
		pkgName := n.PkgName
		if pkgName == "" || (n.PkgPath != "" && n.PkgPath != pkgPath) {
			pkgName = lastSegment(pkgPath)
		}
		if pkgName == "" {
			// As a last resort default to "main" to produce valid code.
			pkgName = "main"
		}
		gf := &GoFile{PkgName: pkgName}
		// Attach types in stable order.
		names := make([]string, 0, len(byName))
		for name := range byName {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			t := byName[name]
			gf.AddType(t)
		}
		files[pkgPath] = gf
	}
	return files, nil
}

package model

// Package represents a loaded Go package used by the synthetic type
// utilities. It intentionally models only the minimal shape required by
// the tests and loader in this package.
type Package struct {
	// Name is the package name as declared in the source files.
	Name string
	// PkgPath is the full import path of the package, e.g.
	// "example.com/project/pkg".
	PkgPath string

	// Types lists all discovered type declarations in the package.
	Types []*Type

	// Imports lists unique imports used across the package.
	Imports []ImportRef

	// Files holds optional file-level groupings when callers organise
	// declarations per file.
	Files  []*GoFile
	Consts []ConstDecl
	Vars   []VarDecl

	// Dependencies lists non-standard-library packages imported by this
	// package that were recursively loaded via LoadPackageDeep.
	Dependencies []*Package

	// Funcs holds free-standing functions declared in this package.
	Funcs []*Function
}

// FunctionsByFile groups functions by source filename when available.
func (p *Package) FunctionsByFile() map[string][]*Function {
	out := map[string][]*Function{}
	for _, fn := range p.Funcs {
		if fn == nil {
			continue
		}
		out[fn.File] = append(out[fn.File], fn)
	}
	return out
}

// HasImport reports whether an import with the given path/alias exists.
func (p *Package) HasImport(path, alias string) bool {
	if p == nil || len(p.Imports) == 0 {
		return false
	}
	key := path + " " + alias
	for _, r := range p.Imports {
		if r.Key() == key {
			return true
		}
	}
	return false
}

// AddImport inserts an import reference if it is not already present.
func (p *Package) AddImport(ref ImportRef) {
	if p.HasImport(ref.Path, ref.Alias) {
		return
	}
	p.Imports = append(p.Imports, ref)
}

// HasType reports whether the package contains a type with the given name.
func (p *Package) HasType(name string) bool {
	if p == nil || name == "" {
		return false
	}
	for _, t := range p.Types {
		if t != nil && t.Name == name {
			return true
		}
	}
	return false
}

// AddType appends a type to the package and merges its imports.
func (p *Package) AddType(t *Type) {
	if t == nil {
		return
	}
	p.Types = append(p.Types, t)
	for _, ref := range t.Imports {
		if ref == nil {
			continue
		}
		p.AddImport(*ref)
	}
}

// AddFile appends a GoFile to the package and merges its imports.
func (p *Package) AddFile(f *GoFile) {
	if f == nil {
		return
	}
	p.Files = append(p.Files, f)
	for _, r := range f.Imports {
		p.AddImport(r)
	}
}

// HasConst reports whether a const with the given name exists in the package.
func (p *Package) HasConst(name string) bool {
	if p == nil || name == "" {
		return false
	}
	for _, c := range p.Consts {
		if c.Name == name {
			return true
		}
	}
	return false
}

// HasVar reports whether a var with the given name exists in the package.
func (p *Package) HasVar(name string) bool {
	if p == nil || name == "" {
		return false
	}
	for _, v := range p.Vars {
		if v.Name == name {
			return true
		}
	}
	return false
}

// AddConst adds a package-level constant if not already present by name
// and merges its imports.
func (p *Package) AddConst(c ConstDecl) {
	if p.HasConst(c.Name) {
		return
	}
	p.Consts = append(p.Consts, c)
	for _, r := range c.Imports {
		p.AddImport(r)
	}
}

// AddVar adds a package-level variable if not already present by name
// and merges its imports.
func (p *Package) AddVar(v VarDecl) {
	if p.HasVar(v.Name) {
		return
	}
	p.Vars = append(p.Vars, v)
	for _, r := range v.Imports {
		p.AddImport(r)
	}
}

// FileByName returns the first file with the given Name, or nil if none.
func (p *Package) FileByName(name string) *GoFile {
	if p == nil || name == "" {
		return nil
	}
	for _, f := range p.Files {
		if f != nil && f.Name == name {
			return f
		}
	}
	return nil
}

// AddOrGetFile returns a file with the given name, creating and adding a
// new file with provided package name when missing.
func (p *Package) AddOrGetFile(name, pkgName string) *GoFile {
	if name == "" {
		return nil
	}
	if f := p.FileByName(name); f != nil {
		return f
	}
	f := &GoFile{Name: name, PkgName: pkgName}
	p.AddFile(f)
	return f
}

const DefaultFileName = "types_gen.go"

// DefaultFile returns (and creates if missing) a default file for this package.
func (p *Package) DefaultFile(pkgName string) *GoFile {
	return p.AddOrGetFile(DefaultFileName, pkgName)
}

// AddDependency inserts a dependency if not already present (by PkgPath).
func (p *Package) AddDependency(dep *Package) {
	if dep == nil || dep.PkgPath == "" {
		return
	}
	for _, d := range p.Dependencies {
		if d != nil && d.PkgPath == dep.PkgPath {
			return
		}
	}
	p.Dependencies = append(p.Dependencies, dep)
}

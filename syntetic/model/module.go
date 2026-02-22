package model

// Module models a Go module containing one or more packages.
// It is intentionally minimal and focused on helper methods for
// composition and de-duplication.
type Module struct {
	Path     string
	Packages map[string]*Package // key: package import path
}

// AddPackage registers p under its PkgPath and returns the stored reference.
// If a package with the same PkgPath exists, it returns the existing one.
func (m *Module) AddPackage(p *Package) *Package {
	if p == nil || p.PkgPath == "" {
		return p
	}
	if m.Packages == nil {
		m.Packages = map[string]*Package{}
	}
	if existing, ok := m.Packages[p.PkgPath]; ok {
		return existing
	}
	m.Packages[p.PkgPath] = p
	return p
}

// HasPackage reports whether a package with the given import path exists.
func (m *Module) HasPackage(pkgPath string) bool {
	if m == nil || pkgPath == "" || len(m.Packages) == 0 {
		return false
	}
	_, ok := m.Packages[pkgPath]
	return ok
}

// AddOrGetPackage returns a package for pkgPath, creating and registering
// a new one when it does not already exist.
func (m *Module) AddOrGetPackage(pkgPath string) *Package {
	if pkgPath == "" {
		return nil
	}
	if m.Packages == nil {
		m.Packages = map[string]*Package{}
	}
	if p, ok := m.Packages[pkgPath]; ok {
		return p
	}
	p := &Package{PkgPath: pkgPath}
	m.Packages[pkgPath] = p
	return p
}

// Package returns the package by import path if present, otherwise nil.
func (m *Module) Package(pkgPath string) *Package {
	if m == nil || pkgPath == "" {
		return nil
	}
	return m.Packages[pkgPath]
}

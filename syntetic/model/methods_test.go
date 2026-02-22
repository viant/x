package model

import "testing"

func TestStructAddFieldDedup(t *testing.T) {
	s := &Struct{}
	s.AddField(Field{Name: "ID"})
	s.AddField(Field{Name: "ID"})
	if got := len(s.Fields); got != 1 {
		t.Fatalf("expected 1 field after dedup, got %d", got)
	}
}

func TestInterfaceAddMethodDedup(t *testing.T) {
	i := &Interface{}
	i.AddMethod(Method{Name: "Do"})
	i.AddMethod(Method{Name: "Do"})
	if got := len(i.Methods); got != 1 {
		t.Fatalf("expected 1 method after dedup, got %d", got)
	}
}

func TestPackageAddTypeAndFileDedupImports(t *testing.T) {
	p := &Package{}
	p.AddImport(ImportRef{Path: "time"})
	p.AddImport(ImportRef{Path: "time"})
	if got := len(p.Imports); got != 1 {
		t.Fatalf("expected 1 import after dedup, got %d", got)
	}

	f := &GoFile{Imports: map[string]ImportRef{"time ": {Path: "time"}}}
	p.AddFile(f)
	if got := len(p.Imports); got != 1 {
		t.Fatalf("expected still 1 import after merging file, got %d", got)
	}
}

func TestModuleAddPackageAndHas(t *testing.T) {
	m := &Module{}
	p := &Package{PkgPath: "example.com/mod/pkg"}
	m.AddPackage(p)
	m.AddPackage(&Package{PkgPath: "example.com/mod/pkg"})
	if !m.HasPackage("example.com/mod/pkg") {
		t.Fatalf("expected to have package")
	}
	if got := len(m.Packages); got != 1 {
		t.Fatalf("expected 1 package stored, got %d", got)
	}
}

func TestModuleAddOrGetPackageAndPackageGetter(t *testing.T) {
	m := &Module{}
	p1 := m.AddOrGetPackage("example.com/mod/alpha")
	if p1 == nil || p1.PkgPath != "example.com/mod/alpha" {
		t.Fatalf("unexpected package: %#v", p1)
	}
	p2 := m.AddOrGetPackage("example.com/mod/alpha")
	if p1 != p2 {
		t.Fatalf("expected same pointer for repeated AddOrGetPackage")
	}
	if got := m.Package("example.com/mod/alpha"); got != p1 {
		t.Fatalf("Package getter mismatch")
	}
}

func TestPackageFileByNameAndAddOrGetFile(t *testing.T) {
	p := &Package{}
	if p.FileByName("types_gen.go") != nil {
		t.Fatalf("expected nil for missing file")
	}
	f1 := p.AddOrGetFile("types_gen.go", "example")
	if f1 == nil || f1.Name != "types_gen.go" || f1.PkgName != "example" {
		t.Fatalf("unexpected file: %#v", f1)
	}
	f2 := p.AddOrGetFile("types_gen.go", "example")
	if f2 != f1 {
		t.Fatalf("expected same pointer for repeated AddOrGetFile")
	}
}

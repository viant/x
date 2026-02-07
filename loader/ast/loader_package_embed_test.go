//go:build viant_afs

package ast

import (
	"context"
	"testing"

	"github.com/viant/x/syntetic/model"
)

// Data‑driven tests for //go:embed detection and embed.FS instantiation.
func TestLoadPackageFS_Embeds_DataDriven(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string]string // rooted at 'root/'
		dir    string            // package dir under root
		verify func(t *testing.T, pkgPath string, got *PackageResult)
	}{
		{
			name: "single pattern embed",
			files: map[string]string{
				"root/go.mod":   "module example.com/emb\n\n",
				"root/p/a.txt":  "hello",
				"root/p/b.txt":  "world",
				"root/p/emb.go": "package p\n\nimport \"embed\"\n//go:embed a.txt\nvar FS embed.FS\n",
			},
			dir: "root/p",
			verify: func(t *testing.T, pkgPath string, got *PackageResult) {
				t.Helper()
				if pkgPath != "example.com/emb/p" {
					t.Fatalf("unexpected pkgPath: %s", pkgPath)
				}
				// Locate file 'emb.go' and verify embed map entry exists for var FS
				f := got.FileByName("emb.go")
				if f == nil || f.Embeds == nil {
					t.Fatalf("missing embed mapping for emb.go: %#v", f)
				}
				if _, ok := f.Embeds["FS"]; !ok {
					t.Fatalf("expected embed FS for var FS in emb.go")
				}
			},
		},
		{
			name: "multi‑pattern embed",
			files: map[string]string{
				"root/go.mod":         "module example.com/emb\n\n",
				"root/p/static/a.txt": "A",
				"root/p/static/b.txt": "B",
				"root/p/static/c.dat": "C",
				"root/p/emb2.go":      "package p\n\nimport \"embed\"\n//go:embed static/*.txt\nvar Web embed.FS\n",
			},
			dir: "root/p",
			verify: func(t *testing.T, pkgPath string, got *PackageResult) {
				t.Helper()
				f := got.FileByName("emb2.go")
				if f == nil || f.Embeds == nil {
					t.Fatalf("missing embed mapping for emb2.go")
				}
				if _, ok := f.Embeds["Web"]; !ok {
					t.Fatalf("expected embed FS for var Web in emb2.go")
				}
			},
		},
	}

	ctx := context.Background()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fsys := MkFS(t, tc.files)
			pkg, err := LoadPackageFS(ctx, fsys, tc.dir)
			if err != nil {
				t.Fatalf("LoadPackageFS error: %v", err)
			}
			// lightweight view for verification
			pr := NewPackageResult(pkg)
			tc.verify(t, pkg.PkgPath, pr)
		})
	}
}

// PackageResult is a small test‑only view to simplify assertions.
type PackageResult struct {
	Name  string
	Path  string
	Files []*FileResult
}

type FileResult struct {
	Name   string
	Embeds map[string]any // var name -> instance (opaque)
}

func NewPackageResult(p *model.Package) *PackageResult {
	out := &PackageResult{Name: p.Name, Path: p.PkgPath}
	for _, gf := range p.Files {
		fr := &FileResult{Name: gf.Name}
		if gf.Embeds != nil {
			fr.Embeds = map[string]any{}
			for vname, inst := range gf.Embeds {
				fr.Embeds[vname] = inst
			}
		}
		out.Files = append(out.Files, fr)
	}
	return out
}

func (p *PackageResult) FileByName(name string) *FileResult {
	for _, f := range p.Files {
		if f.Name == name {
			return f
		}
	}
	return nil
}

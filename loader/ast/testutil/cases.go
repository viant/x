package testutil

import (
	"bytes"
	"context"
	"go/parser"
	"go/printer"
	"go/token"
	"testing"

	ast2 "github.com/viant/x/loader/ast"
	"github.com/viant/x/syntetic/model"
)

// Case defines a data‑driven loader test input and expectations.
type Case struct {
	Name   string
	Files  map[string]string                    // rooted at a logical 'root/' dir
	Dir    string                               // package directory under root
	Expect *Expect                              // optional declarative expectations
	Verify func(t *testing.T, p *model.Package) // optional custom validation
}

// Expect defines declarative assertions for a loaded package.
type Expect struct {
	PkgPath string
	Types   []string
	Consts  []string
	Vars    []string
	// Body maps type name -> expected RHS type expression (normalized via go/printer).
	Body map[string]string
	// Embeds maps file name -> embedded var names expected in that file.
	Embeds map[string][]string
}

// Run executes the case against the loader.
func (c *Case) Run(t *testing.T) {
	t.Helper()
	fsys := ast2.MkFS(t, c.Files)
	ctx := context.Background()
	p, err := ast2.LoadPackageFS(ctx, fsys, c.Dir)
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	if c.Expect != nil {
		c.Expect.assert(t, p)
	}
	if c.Verify != nil {
		c.Verify(t, p)
	}
}

func (e *Expect) assert(t *testing.T, p *model.Package) {
	t.Helper()
	if e.PkgPath != "" && p.PkgPath != e.PkgPath {
		t.Fatalf("unexpected PkgPath: got=%s want=%s", p.PkgPath, e.PkgPath)
	}
	for _, name := range e.Types {
		if !p.HasType(name) {
			t.Fatalf("missing expected type: %s", name)
		}
	}
	for _, name := range e.Consts {
		if !p.HasConst(name) {
			t.Fatalf("missing expected const: %s", name)
		}
	}
	for _, name := range e.Vars {
		if !p.HasVar(name) {
			t.Fatalf("missing expected var: %s", name)
		}
	}
	if len(e.Body) > 0 {
		for typeName, wantExpr := range e.Body {
			var found *model.Type
			for _, tpe := range p.Types {
				if tpe != nil && tpe.Name == typeName {
					found = tpe
					break
				}
			}
			if found == nil {
				t.Fatalf("expected type body for %s but type not found", typeName)
			}
			if normalize(found.Body()) != normalize(wantExpr) {
				t.Fatalf("type %s body mismatch:\n got: %s\nwant: %s", typeName, found.Body(), wantExpr)
			}
		}
	}
	if len(e.Embeds) > 0 {
		for fileName, vars := range e.Embeds {
			gf := fileByName(p, fileName)
			if gf == nil {
				t.Fatalf("expected file %s for embed check", fileName)
			}
			for _, v := range vars {
				if !gf.HasEmbed(v) {
					t.Fatalf("expected embed var %s in file %s", v, fileName)
				}
			}
		}
	}
}

func normalize(expr string) string {
	node, err := parser.ParseExpr(expr)
	if err != nil {
		return expr
	}
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, token.NewFileSet(), node)
	return buf.String()
}

func fileByName(p *model.Package, name string) *model.GoFile {
	for _, f := range p.Files {
		if f != nil && f.Name == name {
			return f
		}
	}
	return nil
}

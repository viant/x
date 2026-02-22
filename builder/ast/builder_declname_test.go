package ast

import (
	"go/ast"
	"testing"

	mdl "github.com/viant/x/syntetic/model"
	trf "github.com/viant/x/syntetic/model/transform"
)

func TestBuilderAST_DeclNameOverride(t *testing.T) {
	// Build a simple type with an initial name
	tp := &mdl.Type{PkgPath: "example.com/p", Name: "Orig", TypeSpec: &ast.TypeSpec{Name: ast.NewIdent("Orig"), Type: ast.NewIdent("int")}}
	b := New(WithTransforms(trf.DeclNameOverride(func(t *mdl.Type) (string, bool) {
		if t.Name == "Orig" {
			return "NewName", true
		}
		return "", false
	})))

	spec := b.TypeSpec(tp, nil)
	if spec == nil || spec.Name == nil || spec.Name.Name != "NewName" {
		t.Fatalf("decl name override failed: got %#v", spec)
	}
	decl := b.GenDecl(tp, nil)
	if decl == nil || len(decl.Specs) != 1 {
		t.Fatalf("unexpected decl: %#v", decl)
	}
	if ts, ok := decl.Specs[0].(*ast.TypeSpec); !ok || ts.Name.Name != "NewName" {
		t.Fatalf("decl type spec not overridden: %#v", decl.Specs[0])
	}
}

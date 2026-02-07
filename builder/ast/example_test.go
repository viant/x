package ast_test

import (
	"fmt"
	"go/ast"

	bast "github.com/viant/x/builder/ast"
	mdl "github.com/viant/x/syntetic/model"
	trf "github.com/viant/x/syntetic/model/transform"
)

// Example: declaration-name override without mutating the model.Type.
func Example_declNameOverride() {
	t := &mdl.Type{PkgPath: "example.com/p", Name: "Orig", TypeSpec: &ast.TypeSpec{Name: ast.NewIdent("Orig"), Type: ast.NewIdent("int")}}
	b := bast.New(bast.WithTransforms(trf.DeclNameOverride(func(tp *mdl.Type) (string, bool) {
		if tp.Name == "Orig" {
			return "NewName", true
		}
		return "", false
	})))
	spec := b.TypeSpec(t, nil)
	fmt.Println(spec.Name.Name)
	// Output: NewName
}

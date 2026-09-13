package x

import (
	"go/ast"
	"reflect"
	"strings"
	"testing"

	"github.com/viant/x/syntetic/model"
)

func TestClonerDetachesSyntheticPayloadAndCaches(t *testing.T) {
	for _, body := range []ast.Expr{ast.NewIdent("string"), &ast.StructType{Fields: &ast.FieldList{List: []*ast.Field{{Names: []*ast.Ident{ast.NewIdent("ID")}, Type: ast.NewIdent("int")}}}}} {
		source := &Type{Name: "Original", PkgPath: "example.com/p", Type: reflect.TypeOf(""), SynteticType: &model.Type{Name: "Original", TypeSpec: &ast.TypeSpec{Name: ast.NewIdent("Original"), Type: body}, Imports: map[string]*model.ImportRef{"x": {Path: "example.com/x"}}}}
		_ = source.Key()
		_ = source.SynteticType.Body()
		copy, err := (Cloner{}).Type(source)
		if err != nil {
			t.Fatal(err)
		}
		copy.Name = "Changed"
		copy.SynteticType.TypeSpec.Type = ast.NewIdent("bool")
		copy.SynteticType.Imports["x"].Path = "changed"
		if copy.Key() == source.Key() || copy.SynteticType.Body() != "bool" || source.SynteticType.Imports["x"].Path != "example.com/x" || copy.Type != source.Type {
			t.Fatal("clone aliases mutable payload or cached identity")
		}
	}
}

func TestClonerPreservesCyclesWithoutSourceAliases(t *testing.T) {
	identifier := ast.NewIdent("Node")
	object := &ast.Object{Name: "Node", Decl: identifier}
	identifier.Obj = object
	source := &model.Type{Name: "Node", TypeSpec: &ast.TypeSpec{Name: identifier, Type: identifier}}
	copy, err := (Cloner{}).Synthetic(source)
	if err != nil {
		t.Fatal(err)
	}
	if copy.TypeSpec.Name == identifier || copy.TypeSpec.Name.Obj == object || copy.TypeSpec.Name.Obj.Decl != copy.TypeSpec.Name || copy.TypeSpec.Type != copy.TypeSpec.Name {
		t.Fatal("cycle/alias structure not detached")
	}
	copy.TypeSpec.Name.Obj.Data = make(chan int)
	if _, err := (Cloner{}).Synthetic(copy); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatal("unsupported payload not rejected")
	}
}

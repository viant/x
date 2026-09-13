package shape

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"

	"github.com/viant/x"
	"github.com/viant/x/syntetic/model"
)

type implementationHook interface{ Apply(context.Context) error }
type implementationPrivate interface{ private() }
type implementationReceiver struct{}

func (*implementationReceiver) Apply(context.Context) error { return nil }
func (implementationReceiver) private()                     {}

type implementationPointer *implementationReceiver

func TestTypeImplementsLinked(t *testing.T) {
	for _, tc := range []struct {
		receiver      reflect.Type
		contract      reflect.Type
		pointer, want bool
	}{
		{reflect.TypeOf((*implementationReceiver)(nil)).Elem(), reflect.TypeOf((*implementationHook)(nil)).Elem(), true, true},
		{reflect.TypeOf((**implementationReceiver)(nil)).Elem(), reflect.TypeOf((*implementationHook)(nil)).Elem(), false, false},
		{reflect.TypeOf((*implementationPointer)(nil)).Elem(), reflect.TypeOf((*implementationHook)(nil)).Elem(), true, false},
		{reflect.TypeOf((*implementationReceiver)(nil)).Elem(), reflect.TypeOf((*implementationPrivate)(nil)).Elem(), false, true},
		{reflect.TypeOf((*int)(nil)).Elem(), reflect.TypeOf((*any)(nil)).Elem(), false, true},
	} {
		got, err := Linked(tc.receiver).Implements(tc.contract, tc.pointer)
		if err != nil || got != tc.want {
			t.Fatalf("%v implements %v=%v error=%v", tc.receiver, tc.contract, got, err)
		}
	}
	if _, err := Linked(reflect.TypeOf((*int)(nil)).Elem()).Implements(reflect.TypeOf((*int)(nil)).Elem(), false); err == nil {
		t.Fatal("noninterface accepted")
	}
	var missing *Type
	if _, err := missing.Implements(reflect.TypeOf((*any)(nil)).Elem(), false); err == nil {
		t.Fatal("missing authority accepted")
	}
}

func TestTypeImplementsSynthetic(t *testing.T) {
	for _, tc := range []struct {
		signature     string
		pointer, want bool
	}{
		{"Apply(ctx c.Context)error", true, true},
		{"Apply(ctx c.Context)error", false, false},
		{"Apply(ctx c.Context)bool", true, false},
		{"Apply(ctx ...c.Context)error", true, false},
	} {
		file, err := parser.ParseFile(token.NewFileSet(), "methods.go", "package sample\nfunc(*Source)"+tc.signature+"{panic(0)}", 0)
		if err != nil {
			t.Fatal(err)
		}
		descriptor := &x.Type{Name: "Source", PkgPath: "example.com/sample", SynteticType: &model.Type{Name: "Source", PkgPath: "example.com/sample", TypeSpec: &ast.TypeSpec{Name: ast.NewIdent("Source"), Type: &ast.StructType{Fields: &ast.FieldList{}}}, Imports: map[string]*model.ImportRef{"c": {Path: "context"}}, PtrMethodsAST: []*ast.FuncDecl{file.Decls[0].(*ast.FuncDecl)}}}
		shape := New(descriptor, nil)
		got, err := shape.Implements(reflect.TypeOf((*implementationHook)(nil)).Elem(), tc.pointer)
		if err != nil || got != tc.want {
			t.Fatalf("%s pointer=%v result=%v error=%v", tc.signature, tc.pointer, got, err)
		}
		if _, err := shape.Implements(reflect.TypeOf((*implementationPrivate)(nil)).Elem(), true); err == nil {
			t.Fatal("synthetic private contract guessed")
		}
	}
}

package shape

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"

	"github.com/viant/x"
	model "github.com/viant/x/syntetic/model"
)

type methodState[T, P any] struct {
	Current *T
	Parent  *P
}
type methodRecord struct{}
type methodParent struct{}
type methodHook struct{}

func (methodHook) Init(context.Context, *methodRecord, methodState[methodRecord, methodParent]) error {
	return nil
}
func (*methodHook) Validate(context.Context, *methodRecord, methodState[methodRecord, methodParent]) error {
	return nil
}

func TestMethodsLinkedAndSyntheticCanonicalIdentity(t *testing.T) {
	linked := Linked(reflect.TypeOf(methodHook{}))
	value, err := linked.Methods(false)
	if err != nil || len(value) != 1 {
		t.Fatalf("value methods = %+v, %v", value, err)
	}
	pointer, err := linked.Methods(true)
	if err != nil || len(pointer) != 2 {
		t.Fatalf("pointer methods = %+v, %v", pointer, err)
	}
	location := "github.com/viant/x/shape"
	want := Method{Name: "Init", Parameters: []string{"context.Context", "*" + location + ".methodRecord", location + ".methodState[" + location + ".methodRecord," + location + ".methodParent]"}, Results: []string{"error"}}
	if !reflect.DeepEqual(value[0], want) {
		t.Fatalf("method = %#v, want %#v", value[0], want)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "hooks.go", `package shape
func(Hook)Init(ctx ctxalias.Context,current *methodRecord,state methodState[methodRecord,methodParent])error{return nil}
func(*Hook)Validate(ctx ctxalias.Context,current *methodRecord,state methodState[methodRecord,methodParent])error{return nil}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := &x.Type{Name: "Hook", PkgPath: location, SynteticType: &model.Type{Name: "Hook", PkgPath: location, Imports: map[string]*model.ImportRef{"ctxalias": {Path: "context"}}, MethodsAST: []*ast.FuncDecl{file.Decls[0].(*ast.FuncDecl)}, PtrMethodsAST: []*ast.FuncDecl{file.Decls[1].(*ast.FuncDecl)}}}
	actual, err := New(descriptor, nil).Methods(true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, pointer) {
		t.Fatalf("synthetic = %#v, linked = %#v", actual, pointer)
	}
	actual[0].Parameters[0] = "mutated"
	again, err := New(descriptor, nil).Methods(true)
	if err != nil || again[0].Parameters[0] != "context.Context" {
		t.Fatal("method result aliases descriptor")
	}
}

func TestMethodsSyntheticModelAndVariadic(t *testing.T) {
	descriptor := &x.Type{Name: "Hooks", PkgPath: "example.com/one", SynteticType: &model.Type{Name: "Hooks", PkgPath: "example.com/one", Methods: model.MethodSet{Value: []model.Method{{Name: "Apply", Type: model.Func{Params: []model.Field{{Type: &model.Basic{Name: "string"}}}, Results: []model.Field{{Type: &model.Basic{Name: "error"}}}, Variadic: true}}}}}}
	actual, err := New(descriptor, nil).Methods(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(actual) != 1 || !actual[0].Variadic || !reflect.DeepEqual(actual[0].Parameters, []string{"[]string"}) {
		t.Fatalf("model methods = %#v", actual)
	}
}

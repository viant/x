package ast

import (
	"context"
	"strings"
	"testing"

	"github.com/viant/x/syntetic/model"
)

func TestLoadPackageFS_Methods_Binding(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/methods\n\n",
		"root/p/t.go": `package p

type T struct{ X int }

func (t T) Val() int { return t.X }
func (t *T) Ptr() error { return nil }
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	if !pkg.HasType("T") {
		t.Fatalf("expected type T")
	}
	var tpe *model.Type
	for _, tp := range pkg.Types {
		if tp.Name == "T" {
			tpe = tp
			break
		}
	}
	if tpe == nil {
		t.Fatalf("type T not found")
	}
	// Ensure AST capture
	if len(tpe.MethodsAST) != 1 || len(tpe.PtrMethodsAST) != 1 {
		t.Fatalf("expected 1 value and 1 pointer method, got %d/%d", len(tpe.MethodsAST), len(tpe.PtrMethodsAST))
	}
	// Ensure MethodSet binding and signature conversion
	hasVal, hasPtr := false, false
	for _, m := range tpe.Methods.Value {
		if m.Name == "Val" {
			hasVal = true
		}
	}
	for _, m := range tpe.Methods.Pointer {
		if m.Name == "Ptr" {
			hasPtr = true
		}
	}
	if !hasVal || !hasPtr {
		t.Fatalf("expected methods in MethodSet: value=%v pointer=%v", hasVal, hasPtr)
	}
	// Validate converted method signatures
	var valM, ptrM *model.Method
	for i := range tpe.Methods.Value {
		if tpe.Methods.Value[i].Name == "Val" {
			valM = &tpe.Methods.Value[i]
		}
	}
	for i := range tpe.Methods.Pointer {
		if tpe.Methods.Pointer[i].Name == "Ptr" {
			ptrM = &tpe.Methods.Pointer[i]
		}
	}
	if valM == nil || len(valM.Type.Results) != 1 {
		t.Fatalf("Val signature not captured")
	}
	if b, ok := valM.Type.Results[0].Type.(*model.Basic); !ok || b.Name != "int" {
		t.Fatalf("Val result expected int, got %#v", valM.Type.Results[0].Type)
	}
	if ptrM == nil || len(ptrM.Type.Results) != 1 {
		t.Fatalf("Ptr signature not captured")
	}
	if b, ok := ptrM.Type.Results[0].Type.(*model.Basic); !ok || b.Name != "error" {
		t.Fatalf("Ptr result expected error, got %#v", ptrM.Type.Results[0].Type)
	}
	// Render stubs
	stubs := tpe.MethodStubs()
	joined := strings.Join(stubs, "\n")
	if !strings.Contains(joined, "func (t T) Val() int") || !strings.Contains(joined, "func (t *T) Ptr() error") {
		t.Fatalf("rendered stubs missing expected signatures:\n%s", joined)
	}
}

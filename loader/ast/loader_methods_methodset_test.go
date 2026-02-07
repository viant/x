package ast

import (
	"context"
	"testing"
)

// Verifies that MethodSet.Value and MethodSet.Pointer are populated for a
// simple type with value and pointer receiver methods.
func TestLoadPackageFS_Methods_MethodSetPopulated(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/methods\n\n",
		"root/p/t.go": `package p

type T struct{ X int }

func (t T) Val() {}
func (t *T) Ptr() {}
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	var methodsValue, methodsPtr int
	var hasVal, hasPtr bool
	for _, tp := range pkg.Types {
		if tp.Name != "T" {
			continue
		}
		methodsValue = len(tp.Methods.Value)
		methodsPtr = len(tp.Methods.Pointer)
		for _, m := range tp.Methods.Value {
			if m.Name == "Val" {
				hasVal = true
			}
		}
		for _, m := range tp.Methods.Pointer {
			if m.Name == "Ptr" {
				hasPtr = true
			}
		}
		break
	}
	if methodsValue != 1 || methodsPtr != 1 || !hasVal || !hasPtr {
		t.Fatalf("MethodSet population mismatch: value=%d(hasVal=%v) pointer=%d(hasPtr=%v)", methodsValue, hasVal, methodsPtr, hasPtr)
	}
}

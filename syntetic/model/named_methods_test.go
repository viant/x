package model

import "testing"

func TestNamed_AddMethodAndPtrMethod(t *testing.T) {
	// Build a named struct type
	st := &Struct{Fields: []Field{{Name: "ID", Type: &Basic{Name: "int"}}}}
	n := &Named{PkgPath: "example.com/p", Name: "User", Underlying: st}

	// Build a method: func() error
	fnVal, _ := MakeFunc(WithResult(&Basic{Name: "error"}))
	n.AddMethod(Method{Name: "Validate", Type: *fnVal})
	if !n.HasMethod("Validate", false) {
		t.Fatalf("expected value method Validate to be present")
	}

	// Pointer receiver method: func() int
	fnPtr, _ := MakeFunc(WithResult(&Basic{Name: "int"}))
	n.AddPtrMethod(Method{Name: "Inc", Type: *fnPtr})
	if !n.HasMethod("Inc", true) {
		t.Fatalf("expected pointer method Inc to be present")
	}
}

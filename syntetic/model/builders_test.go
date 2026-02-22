package model

import "testing"

func TestBuilders_StructInterfaceFunc(t *testing.T) {
	sb := NewStruct().AddField("ID", BasicType("int"), "", false)
	st := sb.Node()
	if st == nil || !st.HasField("ID") {
		t.Fatalf("struct builder did not add field")
	}

	fb := NewFunc().AddParam(BasicType("int")).AddResult(BasicType("error")).SetVariadic(false)
	fn := fb.Build()
	if fn == nil || len(fn.Params) != 1 || len(fn.Results) != 1 {
		t.Fatalf("func builder unexpected shape: %#v", fn)
	}

	ib := NewInterface().AddMethod("Do", *fn)
	iface := ib.Node()
	if iface == nil || !iface.HasMethod("Do") {
		t.Fatalf("interface builder did not add method")
	}
}

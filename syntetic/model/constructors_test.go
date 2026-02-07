package model

import "testing"

func TestNewFieldAndNewFuncOptions(t *testing.T) {
	intType := BasicType("int")
	f, err := NewField(WithFieldName("ID"), WithFieldType(intType))
	if err != nil || f.Name != "ID" || f.Type != intType {
		t.Fatalf("NewField failed: %#v, err=%v", f, err)
	}

	fn, err := MakeFunc(WithParam(intType), WithResult(BasicType("error")), WithVariadic(false))
	if err != nil || len(fn.Params) != 1 || len(fn.Results) != 1 {
		t.Fatalf("NewFunc failed: %#v, err=%v", fn, err)
	}
}

func TestNewMapArraySlicePointer(t *testing.T) {
	s, err := NewSlice(WithSliceElem(BasicType("string")))
	if err != nil || s.Elem == nil {
		t.Fatalf("NewSlice failed: %#v, %v", s, err)
	}
	a, err := NewArray(WithArrayLen(3), WithArrayElem(BasicType("int")))
	if err != nil || a.Elem == nil || a.Len != 3 {
		t.Fatalf("NewArray failed: %#v, %v", a, err)
	}
	m, err := NewMap(WithMapKey(BasicType("string")), WithMapElem(BasicType("int")))
	if err != nil || m.Key == nil || m.Elem == nil {
		t.Fatalf("NewMap failed: %#v, %v", m, err)
	}
	p, err := NewPointer(WithElem(BasicType("int")))
	if err != nil || p.Elem == nil {
		t.Fatalf("NewPointer failed: %#v, %v", p, err)
	}
}

func TestNewNamedAliasBasic(t *testing.T) {
	b, err := NewBasic(WithBasicName("int"))
	if err != nil || b.Name != "int" {
		t.Fatalf("NewBasic failed: %#v, %v", b, err)
	}
	n, err := NewNamed(WithNamedName("Age"), WithNamedPkg("example.com/x"))
	if err != nil || n.Name != "Age" {
		t.Fatalf("NewNamed failed: %#v, %v", n, err)
	}
	al, err := NewAlias(WithAliasName("ID"), WithAliasPkg("example.com/y"), WithAliasTarget(b))
	if err != nil || al.Name != "ID" || al.Target != b {
		t.Fatalf("NewAlias failed: %#v, %v", al, err)
	}
}

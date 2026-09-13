package shape

import (
	"reflect"
	"testing"
)

func TestIndexedAccessorPreservesAddressability(t *testing.T) {
	type child struct{ Name string }
	type parent struct {
		Children []child
		One      *child
	}
	type input struct {
		Parents []*parent
		Values  []parent
	}
	for _, path := range []string{"Parents/Children", "Values.Children"} {
		accessor, err := Linked(reflect.TypeOf(input{})).IndexedAccessor(path)
		if err != nil {
			t.Fatal(err)
		}
		value := &input{Parents: []*parent{{Children: []child{{Name: "before"}}}}, Values: []parent{{Children: []child{{Name: "before"}}}}}
		field, err := accessor.GetAt(value, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if !field.CanAddr() {
			t.Fatal("lost element addressability")
		}
		field.Addr().Interface().(*child).Name = "after"
		if path == "Parents/Children" && value.Parents[0].Children[0].Name != "after" || path == "Values.Children" && value.Values[0].Children[0].Name != "after" {
			t.Fatal("mutation lost")
		}
		for _, indexes := range [][]int{{}, {-1}, {0, 2}, {0, 0, 1}} {
			if _, err := accessor.GetAt(value, indexes...); err == nil {
				t.Fatalf("invalid indexes %v accepted", indexes)
			}
		}
	}
	accessor, err := Linked(reflect.TypeOf(input{})).IndexedAccessor("Parents.One.Name")
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []*input{{Parents: []*parent{nil}}, {Parents: []*parent{{One: nil}}}} {
		value, err := accessor.GetAt(source, 0)
		if err != nil || value.IsValid() {
			t.Fatalf("nil path=%v,%v", value, err)
		}
	}
}

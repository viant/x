package model

import "testing"

func TestKindOrder(t *testing.T) {
	// Verify stable ordering of Kind enum values.
	tests := []struct {
		name string
		k    Kind
		val  int
	}{
		{"KindUnknown", KindUnknown, 0},
		{"KindBasic", KindBasic, 1},
		{"KindNamed", KindNamed, 2},
		{"KindAlias", KindAlias, 3},
		{"KindPointer", KindPointer, 4},
		{"KindSlice", KindSlice, 5},
		{"KindArray", KindArray, 6},
		{"KindMap", KindMap, 7},
		{"KindChan", KindChan, 8},
		{"KindFunc", KindFunc, 9},
		{"KindInterface", KindInterface, 10},
		{"KindStruct", KindStruct, 11},
	}

	for _, tt := range tests {
		if got := int(tt.k); got != tt.val {
			t.Fatalf("%s = %d, want %d", tt.name, got, tt.val)
		}
	}
}

func TestNodeKinds(t *testing.T) {
	tests := []struct {
		name string
		node Node
		want Kind
	}{
		{"Basic", &Basic{}, KindBasic},
		{"Named", &Named{}, KindNamed},
		{"Alias", &Alias{}, KindAlias},
		{"Pointer", &Pointer{}, KindPointer},
		{"Slice", &Slice{}, KindSlice},
		{"Array", &Array{}, KindArray},
		{"Map", &Map{}, KindMap},
		{"Chan", &Chan{}, KindChan},
		{"Func", &Func{}, KindFunc},
		{"Interface", &Interface{}, KindInterface},
		{"Struct", &Struct{}, KindStruct},
	}

	for _, tt := range tests {
		if got := tt.node.Kind(); got != tt.want {
			t.Errorf("%s.Kind() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

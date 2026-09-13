package shape

import (
	"reflect"
	"testing"
)

func TestAccessorFieldIndex(t *testing.T) {
	type row struct {
		ID       int
		Child    *Embedded
		Children []Embedded
	}
	owner := Linked(reflect.TypeOf(row{}))
	for _, tc := range []struct {
		path    string
		indexed bool
		want    []int
		invalid bool
	}{
		{"Child.ID", false, []int{1, 0}, false},
		{"Child.ID", true, []int{1, 0}, false},
		{"Children.ID", true, nil, true},
		{"Children", true, []int{2}, false},
	} {
		var accessor *Accessor
		var err error
		if tc.indexed {
			accessor, err = owner.IndexedAccessor(tc.path)
		} else {
			accessor, err = owner.Accessor(tc.path)
		}
		if err != nil {
			t.Fatal(err)
		}
		actual, err := accessor.FieldIndex()
		if (err != nil) != tc.invalid || !tc.invalid && !reflect.DeepEqual(actual, tc.want) {
			t.Fatalf("%s: index=%v err=%v", tc.path, actual, err)
		}
		if len(actual) > 0 {
			actual[0] = 99
			again, _ := accessor.FieldIndex()
			if !reflect.DeepEqual(again, tc.want) {
				t.Fatal("index alias escaped")
			}
		}
	}
	if _, err := (*Accessor)(nil).FieldIndex(); err == nil {
		t.Fatal("nil accessor accepted")
	}
}

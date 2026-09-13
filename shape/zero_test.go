package shape

import (
	"reflect"
	"testing"
)

func TestWithZeroFieldsCopyOnWrite(t *testing.T) {
	type flags struct{ ID, Name bool }
	type filter struct {
		ID   int
		Name string
	}
	type input struct {
		Filter *filter
		Has    *flags
		Shared *filter
	}
	for _, tc := range []struct {
		name     string
		source   *input
		paths    []string
		wantID   int
		wantFlag bool
	}{
		{"value and marker", &input{Filter: &filter{7, "keep"}, Has: &flags{true, true}, Shared: &filter{9, "shared"}}, []string{"Filter.ID", "Has.ID"}, 0, false},
		{"nil branch", &input{Has: &flags{true, true}}, []string{"Filter.ID"}, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy, err := (Runtime{}).WithZeroFields(tc.source, tc.paths...)
			if err != nil {
				t.Fatal(err)
			}
			actual := copy.(*input)
			if actual == tc.source {
				t.Fatal("root aliases source")
			}
			if tc.source.Filter != nil && (actual.Filter == tc.source.Filter || actual.Filter.ID != tc.wantID || tc.source.Filter.ID != 7 || actual.Filter.Name != "keep") {
				t.Fatal("filter branch was not detached correctly")
			}
			if actual.Has.ID != tc.wantFlag || !tc.source.Has.ID {
				t.Fatal("marker/source changed")
			}
			if actual.Shared != tc.source.Shared {
				t.Fatal("untouched branch unnecessarily changed")
			}
		})
	}
	if _, err := (Runtime{}).WithZeroFields(&input{}, "Missing"); err == nil {
		t.Fatal("missing field accepted")
	}
	if _, err := (Runtime{}).WithZeroFields(3, "ID"); err == nil {
		t.Fatal("scalar accepted")
	}
	if !reflect.DeepEqual((Runtime{}).Indirect(reflect.TypeOf(&input{})), reflect.TypeOf(input{})) {
		t.Fatal("source type changed")
	}
}

func TestNamedDescriptorForContainers(t *testing.T) {
	type rows []*Embedded
	for _, tc := range []struct{ source, want reflect.Type }{{reflect.TypeOf([]*Embedded{}), reflect.TypeOf(Embedded{})}, {reflect.TypeOf(rows{}), reflect.TypeOf(rows{})}, {reflect.TypeOf((*shaped)(nil)), reflect.TypeOf(shaped{})}} {
		actual := Linked(tc.source).NamedDescriptor()
		if actual.Type != tc.want {
			t.Fatalf("named=%s want=%s", actual.Type, tc.want)
		}
	}
}

package shape

import (
	"reflect"
	"testing"
)

func TestWithZeroFieldsPreservesNamedPointers(t *testing.T) {
	type flags struct{ ID, Name bool }
	type pointer *flags
	type outer struct{ Flags pointer }
	source := pointer(&flags{ID: true, Name: true})
	result, err := (Runtime{}).WithZeroFields(source, "ID")
	if err != nil {
		t.Fatal(err)
	}
	if reflect.TypeOf(result) != reflect.TypeOf(source) {
		t.Fatalf("type=%T want=%T", result, source)
	}
	actual := result.(pointer)
	if actual == source || actual.ID || !actual.Name || !source.ID {
		t.Fatal("named pointer copy changed identity/data")
	}
	rootCopy, err := (Runtime{}).WithZeroFields(source)
	if err != nil || reflect.TypeOf(rootCopy) != reflect.TypeOf(source) || rootCopy.(pointer) == source || !rootCopy.(pointer).ID {
		t.Fatalf("root pointer copy=%T error=%v", rootCopy, err)
	}
	var missing pointer
	for _, paths := range [][]string{nil, {"ID"}} {
		cloned, err := (Runtime{}).WithZeroFields(missing, paths...)
		if err != nil || reflect.TypeOf(cloned) != reflect.TypeOf(missing) || cloned.(pointer) != nil {
			t.Fatalf("nil named pointer=%T error=%v", cloned, err)
		}
	}
	nested := &outer{Flags: source}
	result, err = (Runtime{}).WithZeroFields(nested, "Flags.Name")
	if err != nil {
		t.Fatal(err)
	}
	copy := result.(*outer)
	if copy.Flags == source || copy.Flags.Name || !copy.Flags.ID || !source.Name {
		t.Fatal("nested named pointer not detached")
	}
}

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

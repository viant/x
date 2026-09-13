package shape

import (
	"reflect"
	"testing"
)

type OptionalFields struct {
	Name  string
	Value *int
}
type optionalRecord struct {
	*OptionalFields
	Children []*OptionalFields
}

func TestAccessorGetOptional(t *testing.T) {
	accessor, err := Linked(reflect.TypeOf(optionalRecord{})).Accessor("Value")
	if err != nil {
		t.Fatal(err)
	}
	value := 7
	for _, test := range []struct {
		name             string
		input            any
		present, invalid bool
		want             *int
	}{
		{"nil root", nil, false, false, nil},
		{"typed nil root", (*optionalRecord)(nil), false, false, nil},
		{"nil holder", &optionalRecord{}, false, false, nil},
		{"present nil leaf", &optionalRecord{OptionalFields: &OptionalFields{}}, true, false, nil},
		{"present leaf", &optionalRecord{OptionalFields: &OptionalFields{Value: &value}}, true, false, &value},
		{"wrong owner", &OptionalFields{}, false, true, nil},
		{"wrong typed nil owner", (*OptionalFields)(nil), false, true, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			actual, present, err := accessor.GetOptional(test.input)
			if (err != nil) != test.invalid || present != test.present {
				t.Fatalf("present=%v error=%v", present, err)
			}
			if present && actual.Interface().(*int) != test.want {
				t.Fatalf("value=%v", actual)
			}
			if !present && actual.IsValid() {
				t.Fatal("absent/error result published a value")
			}
			if record, ok := test.input.(*optionalRecord); ok && record != nil && test.name == "nil holder" && record.OptionalFields != nil {
				t.Fatal("read allocated holder")
			}
		})
	}
}

func TestAccessorOptionalEqual(t *testing.T) {
	accessor, err := Linked(reflect.TypeOf(optionalRecord{})).Accessor("Name")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name           string
		left, right    any
		equal, invalid bool
	}{
		{"both absent", &optionalRecord{}, (*optionalRecord)(nil), true, false},
		{"absent vs zero", &optionalRecord{}, &optionalRecord{OptionalFields: &OptionalFields{}}, false, false},
		{"equal", &optionalRecord{OptionalFields: &OptionalFields{Name: "same"}}, &optionalRecord{OptionalFields: &OptionalFields{Name: "same"}}, true, false},
		{"different", &optionalRecord{OptionalFields: &OptionalFields{Name: "one"}}, &optionalRecord{OptionalFields: &OptionalFields{Name: "two"}}, false, false},
		{"bad right", nil, &OptionalFields{}, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			equal, err := accessor.Equal(test.left, test.right)
			if (err != nil) != test.invalid || equal != test.equal {
				t.Fatalf("equal=%v error=%v", equal, err)
			}
		})
	}
	indexed, err := Linked(reflect.TypeOf(optionalRecord{})).IndexedAccessor("Children.Name")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := indexed.GetOptional(&optionalRecord{}); err == nil {
		t.Fatal("indexed path silently treated as absent")
	}
	var missing *Accessor
	if _, _, err := missing.GetOptional(nil); err == nil {
		t.Fatal("nil accessor accepted")
	}
}

func TestAccessorSparseEmbeddingSiblingValues(t *testing.T) {
	owner := Linked(reflect.TypeOf(optionalRecord{}))
	before := &optionalRecord{}
	value := 7
	after := &optionalRecord{OptionalFields: &OptionalFields{Value: &value}}
	for _, test := range []struct {
		field   string
		changed bool
	}{{"Name", false}, {"Value", true}} {
		t.Run(test.field, func(t *testing.T) {
			accessor, err := owner.Accessor(test.field)
			if err != nil {
				t.Fatal(err)
			}
			previous, err := accessor.Get(before)
			if err != nil {
				t.Fatal(err)
			}
			current, err := accessor.Get(after)
			if err != nil {
				t.Fatal(err)
			}
			if changed := !reflect.DeepEqual(previous.Interface(), current.Interface()); changed != test.changed {
				t.Fatalf("value changed=%v want=%v", changed, test.changed)
			}
			_, previousPresent, err := accessor.GetOptional(before)
			if err != nil {
				t.Fatal(err)
			}
			_, currentPresent, err := accessor.GetOptional(after)
			if err != nil || previousPresent || !currentPresent {
				t.Fatalf("path presence previous=%v current=%v error=%v", previousPresent, currentPresent, err)
			}
		})
	}
	if before.OptionalFields != nil || after.Name != "" || *after.Value != 7 {
		t.Fatal("comparison mutated source values")
	}
}

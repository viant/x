package shape

import (
	"reflect"
	"testing"
)

type assignmentValues struct{ A, B int }
type assignmentPointer *assignmentValues
type assignmentRecursive *assignmentRecursive
type assignmentTarget struct {
	Left, Right *assignmentValues
	Named       assignmentPointer
	Value       int
}

func TestAssignFieldsAtomicAndAddressStable(t *testing.T) {
	for _, allocated := range []bool{false, true} {
		t.Run(map[bool]string{false: "allocate", true: "retain"}[allocated], func(t *testing.T) {
			target := &assignmentTarget{Value: 7}
			if allocated {
				target.Left = &assignmentValues{A: 1, B: 2}
			}
			before := target.Left
			shape := Linked(reflect.TypeOf(target))
			a, err := shape.Accessor("Left.A")
			if err != nil {
				t.Fatal(err)
			}
			b, err := shape.Accessor("Left.B")
			if err != nil {
				t.Fatal(err)
			}
			if err := (Runtime{}).AssignFields(target, FieldValue{a, 3}, FieldValue{b, "wrong"}); err == nil {
				t.Fatal("wrong type accepted")
			}
			if target.Left != before || (before != nil && (before.A != 1 || before.B != 2)) {
				t.Fatal("failed assignment changed target")
			}
			if err := (Runtime{}).AssignFields(target, FieldValue{a, 3}, FieldValue{b, 4}); err != nil {
				t.Fatal(err)
			}
			if allocated && target.Left != before {
				t.Fatal("existing holder identity changed")
			}
			if target.Left.A != 3 || target.Left.B != 4 || target.Value != 7 {
				t.Fatalf("target=%+v", target)
			}
		})
	}
}

func TestAssignFieldsRejectsAmbiguity(t *testing.T) {
	value := &assignmentValues{A: 1, B: 2}
	target := &assignmentTarget{Left: value, Right: value}
	shape := Linked(reflect.TypeOf(target))
	for _, paths := range [][2]string{{"Left.A", "Left.A"}, {"Left", "Left.A"}, {"Left.A", "Right.A"}} {
		a, err := shape.Accessor(paths[0])
		if err != nil {
			t.Fatal(err)
		}
		b, err := shape.Accessor(paths[1])
		if err != nil {
			t.Fatal(err)
		}
		var first any = 3
		if paths[0] == "Left" {
			first = &assignmentValues{}
		}
		if err := (Runtime{}).AssignFields(target, FieldValue{a, first}, FieldValue{b, 4}); err == nil {
			t.Fatalf("overlap accepted: %v", paths)
		}
		if target.Left != value || target.Right != value || value.A != 1 {
			t.Fatal("rejected overlap mutated target")
		}
	}
	if err := (Runtime{}).AssignFields(target, FieldValue{}); err == nil {
		t.Fatal("nil accessor accepted")
	}
}

func TestAssignFieldsNamedAndNil(t *testing.T) {
	target := &assignmentTarget{}
	shape := Linked(reflect.TypeOf(target))
	a, err := shape.Accessor("Named.A")
	if err != nil {
		t.Fatal(err)
	}
	if err := (Runtime{}).AssignFields(target, FieldValue{a, 9}); err != nil {
		t.Fatal(err)
	}
	if target.Named == nil || target.Named.A != 9 {
		t.Fatal("named holder not allocated")
	}
	if err := (Runtime{}).AssignFields(target, FieldValue{a, nil}); err != nil {
		t.Fatal(err)
	}
	if target.Named.A != 0 {
		t.Fatal("nil assignment did not zero scalar")
	}
	for _, invalid := range []any{nil, (*assignmentTarget)(nil), assignmentTarget{}, new(int)} {
		if err := (Runtime{}).AssignFields(invalid); err == nil {
			t.Fatalf("target %T accepted", invalid)
		}
	}
	var recursive assignmentRecursive
	recursive = &recursive
	if err := (Runtime{}).AssignFields(recursive); err == nil {
		t.Fatal("recursive pointer target accepted")
	}
}

func TestAssignFieldsRejectsAliasedAncestor(t *testing.T) {
	type root struct {
		Value assignmentValues
		Alias *assignmentValues
	}
	for _, reverse := range []bool{false, true} {
		target := &root{Value: assignmentValues{A: 1}}
		target.Alias = &target.Value
		shape := Linked(reflect.TypeOf(target))
		a, err := shape.Accessor("Value")
		if err != nil {
			t.Fatal(err)
		}
		b, err := shape.Accessor("Alias.A")
		if err != nil {
			t.Fatal(err)
		}
		fields := []FieldValue{{a, assignmentValues{}}, {b, 9}}
		if reverse {
			fields[0], fields[1] = fields[1], fields[0]
		}
		if err := (Runtime{}).AssignFields(target, fields...); err == nil {
			t.Fatal("aliased ancestor accepted")
		}
		if target.Value.A != 1 {
			t.Fatal("failed preflight changed target")
		}
	}
}

func TestAssignFieldsRejectsPointerHolderAlias(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		value := &assignmentValues{A: 1}
		target := &assignmentTarget{Left: value, Right: value}
		shape := Linked(reflect.TypeOf(target))
		a, err := shape.Accessor("Left")
		if err != nil {
			t.Fatal(err)
		}
		b, err := shape.Accessor("Right.A")
		if err != nil {
			t.Fatal(err)
		}
		fields := []FieldValue{{a, &assignmentValues{A: 10}}, {b, 4}}
		if reverse {
			fields[0], fields[1] = fields[1], fields[0]
		}
		if err := (Runtime{}).AssignFields(target, fields...); err == nil {
			t.Fatal("pointer holder alias accepted")
		}
		if target.Left != value || target.Right != value || value.A != 1 {
			t.Fatal("rejected alias modified target")
		}
	}
}

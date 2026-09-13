package shape

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

type selectedCloneEntity struct {
	Name     string
	Children []selectedCloneEntity
	Next     *selectedCloneEntity
	Has      *struct{ Name bool }
	Service  chan int
	private  chan int
	Mutex    sync.Mutex
}

func TestCloneSelectedEntityCycles(t *testing.T) {
	typ := reflect.TypeOf(selectedCloneEntity{})
	options := CloneOptions{Fields: map[reflect.Type][]string{typ: {"Name", "Children", "Next", "Has"}}}
	t.Run("value slice cycle and full capacity", func(t *testing.T) {
		source := make([]selectedCloneEntity, 1, 2)
		source[0].Name = "root"
		source[0].Children = source
		source[0].Service, source[0].private = make(chan int), make(chan int)
		source[0].Has = &struct{ Name bool }{true}
		source[:2][1].Name = "hidden"
		cloned, err := (Runtime{}).CloneValue(source, options)
		if err != nil {
			t.Fatal(err)
		}
		result := cloned.([]selectedCloneEntity)
		if cap(result) != 2 || result[:2][1].Name != "hidden" || result[0].Service != nil || result[0].private != nil {
			t.Fatal("selection/capacity was not preserved")
		}
		result[0].Children[0].Name = "changed"
		result[0].Has.Name = false
		if result[0].Name != "changed" || source[0].Name != "root" || !source[0].Has.Name {
			t.Fatal("cycle or detached marker was not preserved")
		}
	})
	t.Run("pointer cycle", func(t *testing.T) {
		source := &selectedCloneEntity{Name: "root", Service: make(chan int)}
		source.Next = source
		cloned, err := (Runtime{}).CloneValue(source, options)
		if err != nil {
			t.Fatal(err)
		}
		result := cloned.(*selectedCloneEntity)
		if result == source || result.Next != result || result.Service != nil {
			t.Fatal("pointer cycle selection failed")
		}
	})
}

func TestCloneFieldSelectionValidation(t *testing.T) {
	type embedded struct{ Public int }
	type outer struct{ embedded }
	typ := reflect.TypeOf(selectedCloneEntity{})
	for _, test := range []struct {
		name   string
		typ    reflect.Type
		fields []string
	}{
		{"nil type", nil, nil},
		{"pointer type", reflect.PointerTo(typ), []string{"Name"}},
		{"unknown", typ, []string{"Missing"}},
		{"private", typ, []string{"private"}},
		{"promoted", reflect.TypeOf(outer{}), []string{"Public"}},
		{"synchronization", reflect.TypeOf(sync.Mutex{}), nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := (Runtime{}).CloneValue(nil, CloneOptions{Fields: map[reflect.Type][]string{test.typ: test.fields}})
			if err == nil || result != nil {
				t.Fatalf("result=%v error=%v", result, err)
			}
		})
	}
	if _, err := (Runtime{}).CloneValue(nil, CloneOptions{}, CloneOptions{}); err == nil {
		t.Fatal("ambiguous options accepted")
	}
}

func TestCloneFieldSelectionDefaultsAndEmpty(t *testing.T) {
	source := &selectedCloneEntity{Name: "source", Service: make(chan int)}
	if _, err := (Runtime{}).CloneValue(source); err == nil {
		t.Fatal("default silently skipped unsupported fields")
	}
	result, err := (Runtime{}).CloneValue(source, CloneOptions{Fields: map[reflect.Type][]string{reflect.TypeOf(selectedCloneEntity{}): nil}})
	if err != nil || !reflect.DeepEqual(result, &selectedCloneEntity{}) {
		t.Fatalf("empty selection=%v error=%v", result, err)
	}
	now := time.Now()
	full, err := (Runtime{}).CloneValue(now, CloneOptions{})
	if err != nil || !reflect.DeepEqual(full, now) {
		t.Fatal("default immutable time changed")
	}
	empty, err := (Runtime{}).CloneValue(now, CloneOptions{Fields: map[reflect.Type][]string{reflect.TypeOf(time.Time{}): nil}})
	if err != nil || empty != (time.Time{}) {
		t.Fatalf("selected time=%v error=%v", empty, err)
	}
}

func TestCloneSelectionCannotLeakThroughPrivateField(t *testing.T) {
	type child struct{ Public int }
	type parent struct{ private child }
	source := parent{private: child{Public: 9}}
	if _, err := (Runtime{}).CloneValue(source); err != nil {
		t.Fatalf("normal immutable private field: %v", err)
	}
	result, err := (Runtime{}).CloneValue(source, CloneOptions{Fields: map[reflect.Type][]string{reflect.TypeOf(child{}): nil}})
	if err == nil || result != nil {
		t.Fatalf("private field bypassed child selection: result=%v error=%v", result, err)
	}
}

type CloneSelectionBase struct {
	ID      int
	Name    string
	Service chan int
	private chan int
}
type CloneSelectionRow struct {
	CloneSelectionBase
	Ptr   *CloneSelectionBase
	Label string
}

func TestCloneOptionsSelectPromotedAndNestedFields(t *testing.T) {
	for _, nilPointer := range []bool{false, true} {
		t.Run(fmt.Sprint("nil pointer=", nilPointer), func(t *testing.T) {
			options := CloneOptions{}
			rowType := reflect.TypeOf(CloneSelectionRow{})
			if err := options.Select(reflect.PointerTo(rowType), "ID", "Ptr.Name"); err != nil {
				t.Fatal(err)
			}
			if err := options.Select(rowType, "Label", "ID"); err != nil {
				t.Fatal(err)
			}
			want := map[reflect.Type][]string{rowType: {"CloneSelectionBase", "Ptr", "Label"}, reflect.TypeOf(CloneSelectionBase{}): {"ID", "Name"}}
			if !reflect.DeepEqual(options.Fields, want) {
				t.Fatalf("selections=%v want=%v", options.Fields, want)
			}
			source := &CloneSelectionRow{CloneSelectionBase: CloneSelectionBase{ID: 7, Name: "base", Service: make(chan int), private: make(chan int)}, Ptr: &CloneSelectionBase{ID: 8, Name: "pointer", Service: make(chan int)}, Label: "label"}
			if nilPointer {
				source.Ptr = nil
			}
			cloned, err := (Runtime{}).CloneValue(source, options)
			if err != nil {
				t.Fatal(err)
			}
			result := cloned.(*CloneSelectionRow)
			if result.ID != 7 || result.Name != "base" || result.Label != "label" || result.Service != nil || result.private != nil {
				t.Fatal("selected embedded data incorrect")
			}
			if nilPointer {
				if result.Ptr != nil {
					t.Fatal("nil holder allocated")
				}
			} else {
				if result.Ptr == source.Ptr || result.Ptr.ID != 8 || result.Ptr.Name != "pointer" || result.Ptr.Service != nil {
					t.Fatal("selected pointer holder incorrect")
				}
			}
		})
	}
}

func TestCloneOptionsSelectFailureAtomicity(t *testing.T) {
	type privateHolder struct{ Public int }
	type privateRow struct{ privateHolder }
	type collectionRow struct{ Items []CloneSelectionBase }
	for _, test := range []struct {
		name  string
		typ   reflect.Type
		paths []string
	}{
		{"missing after valid", reflect.TypeOf(CloneSelectionRow{}), []string{"Label", "Missing"}},
		{"private holder", reflect.TypeOf(privateRow{}), []string{"Public"}},
		{"private leaf", reflect.TypeOf(CloneSelectionRow{}), []string{"CloneSelectionBase.private"}},
		{"collection traversal", reflect.TypeOf(collectionRow{}), []string{"Items.Name"}},
		{"nil type", nil, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			key := reflect.TypeOf(CloneSelectionRow{})
			options := CloneOptions{Fields: map[reflect.Type][]string{key: {"Label"}}}
			if err := options.Select(test.typ, test.paths...); err == nil {
				t.Fatal("invalid selection accepted")
			}
			if !reflect.DeepEqual(options.Fields, map[reflect.Type][]string{key: {"Label"}}) {
				t.Fatal("failed selection changed options")
			}
		})
	}
	var absent *CloneOptions
	if err := absent.Select(reflect.TypeOf(CloneSelectionRow{}), "Label"); err == nil {
		t.Fatal("nil options accepted")
	}
}

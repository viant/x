package shape

import "testing"

type collectionRow struct{ ID int }
type collectionRows []collectionRow
type collectionRowPointer *collectionRow
type collectionPointers []collectionRowPointer
type collectionArray [2]collectionRow
type collectionOther struct{ ID int }
type collectionRecursive *collectionRecursive

func TestCollectionPointers(t *testing.T) {
	a, b := collectionRow{1}, collectionRow{2}
	rows := collectionRows{a, b}
	array := collectionArray{a, b}
	for _, tc := range []struct {
		name      string
		value     any
		ids       []int
		nilResult bool
	}{
		{"nil", nil, nil, true},
		{"nil scalar", (*collectionRow)(nil), nil, true},
		{"nil slice", collectionRows(nil), nil, true},
		{"nil container pointer", (*collectionRows)(nil), nil, true},
		{"empty slice", collectionRows{}, []int{}, false},
		{"value", a, []int{1}, false},
		{"pointer", &a, []int{1}, false},
		{"named pointer", collectionRowPointer(&a), []int{1}, false},
		{"named values", rows, []int{1, 2}, false},
		{"slice pointer", &rows, []int{1, 2}, false},
		{"named pointers", collectionPointers{&a, nil, &b}, []int{1, -1, 2}, false},
		{"array", array, []int{1, 2}, false},
		{"array pointer", &array, []int{1, 2}, false},
		{"pointer array", [2]*collectionRow{&a, nil}, []int{1, -1}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := (Collection[collectionRow]{}).Pointers(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			if (got == nil) != tc.nilResult || len(got) != len(tc.ids) {
				t.Fatalf("rows=%v", got)
			}
			for i, id := range tc.ids {
				if id == -1 {
					if got[i] != nil {
						t.Fatal("nil ordinal lost")
					}
					continue
				}
				if got[i] == nil || got[i].ID != id {
					t.Fatalf("row %d=%v", i, got[i])
				}
			}
		})
	}
}

func TestCollectionPointersAddressability(t *testing.T) {
	rows := collectionRows{{ID: 1}}
	got, err := (Collection[collectionRow]{}).Pointers(rows)
	if err != nil || got[0] != &rows[0] {
		t.Fatalf("slice identity: %v", err)
	}
	array := collectionArray{{ID: 1}, {ID: 2}}
	got, err = (Collection[collectionRow]{}).Pointers(&array)
	if err != nil || got[0] != &array[0] {
		t.Fatalf("array pointer identity: %v", err)
	}
	got, err = (Collection[collectionRow]{}).Pointers(array)
	if err != nil {
		t.Fatal(err)
	}
	got[0].ID = 99
	if array[0].ID != 1 {
		t.Fatal("value array did not copy")
	}
	row := collectionRow{ID: 1}
	got, err = (Collection[collectionRow]{}).Pointers(collectionRowPointer(&row))
	if err != nil || got[0] != &row {
		t.Fatalf("named pointer identity: %v", err)
	}
}

func TestCollectionPointersRejectWrongTypes(t *testing.T) {
	for _, value := range []any{1, (*int)(nil), []int(nil), (*[]int)(nil), []collectionOther{}, []*collectionOther{}, []any{collectionRow{}}, collectionOther{}, map[int]collectionRow{}, collectionRecursive(nil)} {
		got, err := (Collection[collectionRow]{}).Pointers(value)
		if err == nil || got != nil {
			t.Fatalf("accepted %T: %v %v", value, got, err)
		}
	}
}

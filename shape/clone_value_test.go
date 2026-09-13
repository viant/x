package shape

import (
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

type cloneEntity struct {
	ID       *int
	Has      *struct{ ID, Name bool }
	Name     string
	Labels   map[string][]string
	Children []*cloneEntity
	Parent   *cloneEntity
	At       time.Time
}

func TestRuntimeCloneValueEntityGraph(t *testing.T) {
	id := 7
	parent := &cloneEntity{ID: &id, Has: &struct{ ID, Name bool }{true, false}, Name: "original", Labels: map[string][]string{"a": {"value"}}, At: time.Now()}
	child := &cloneEntity{ID: &id, Parent: parent, Labels: parent.Labels}
	parent.Children = []*cloneEntity{child, child, nil}
	cloned, err := (Runtime{}).CloneValue(parent)
	if err != nil {
		t.Fatal(err)
	}
	actual := cloned.(*cloneEntity)
	if actual == parent || actual.Children[0] == child || actual.Children[0] != actual.Children[1] || actual.Children[0].Parent != actual || actual.ID != actual.Children[0].ID {
		t.Fatal("graph identity was not preserved/detached")
	}
	if actual.Children[2] != nil || actual.At != parent.At {
		t.Fatal("nil/time changed")
	}
	*actual.ID = 99
	actual.Has.Name = true
	actual.Children[0].Labels["a"][0] = "changed"
	if *parent.ID != 7 || parent.Has.Name || parent.Labels["a"][0] != "value" {
		t.Fatal("clone mutated original")
	}
	if actual.Labels["a"][0] != "changed" {
		t.Fatal("shared map alias lost")
	}
}

func TestRuntimeCloneValueKinds(t *testing.T) {
	type namedSlice []int
	type privateImmutable struct {
		secret string
		Data   []int
	}
	for _, test := range []struct {
		name  string
		value any
	}{
		{"nil", nil}, {"typed nil", (*int)(nil)}, {"nil slice", []int(nil)}, {"empty slice", []int{}}, {"nil map", map[string]int(nil)},
		{"zero", 0}, {"false", false}, {"array", [2][]int{{1}, {2}}}, {"defined slice", namedSlice{1, 2}},
		{"interface", struct{ Value any }{[]int{1, 2}}}, {"private scalar", privateImmutable{secret: "kept", Data: []int{4}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			actual, err := (Runtime{}).CloneValue(test.value)
			if err != nil {
				t.Fatal(err)
			}
			if reflect.TypeOf(actual) != reflect.TypeOf(test.value) || !reflect.DeepEqual(actual, test.value) {
				t.Fatalf("actual=%#v source=%#v", actual, test.value)
			}
		})
	}
}

func TestRuntimeCloneValueSliceCapacityAliases(t *testing.T) {
	backing := []*int{new(int), new(int), new(int)}
	*backing[2] = 12
	source := struct{ Short, Long []*int }{backing[:1], backing[:2]}
	cloned, err := (Runtime{}).CloneValue(source)
	if err != nil {
		t.Fatal(err)
	}
	actual := cloned.(struct{ Short, Long []*int })
	if len(actual.Short) != 1 || cap(actual.Short) != 3 || len(actual.Long) != 2 || &actual.Short[0] != &actual.Long[0] {
		t.Fatal("slice shape/shared backing lost")
	}
	*actual.Short[:3][2] = 30
	if *backing[2] != 12 || *actual.Long[:3][2] != 30 {
		t.Fatal("hidden capacity not detached/shared")
	}
}

func TestRuntimeCloneValueMapAndSliceCycles(t *testing.T) {
	m := map[string]any{}
	m["self"] = m
	s := make([]any, 1)
	s[0] = s
	for _, value := range []any{m, s} {
		cloned, err := (Runtime{}).CloneValue(value)
		if err != nil {
			t.Fatal(err)
		}
		switch actual := cloned.(type) {
		case map[string]any:
			actual["self"].(map[string]any)["new"] = 1
			if actual["new"] != 1 || m["new"] != nil {
				t.Fatal("map cycle detached incorrectly")
			}
		case []any:
			actual[0].([]any)[0] = "changed"
			if actual[0] != "changed" {
				t.Fatal("slice cycle lost")
			}
			if _, ok := s[0].([]any); !ok {
				t.Fatal("original slice changed")
			}
		}
	}
}

func TestRuntimeCloneValuePointerMapKeys(t *testing.T) {
	key := new(int)
	*key = 3
	source := struct {
		Key  *int
		Rows map[*int]*int
	}{key, map[*int]*int{key: key}}
	value, err := (Runtime{}).CloneValue(source)
	if err != nil {
		t.Fatal(err)
	}
	actual := value.(struct {
		Key  *int
		Rows map[*int]*int
	})
	if actual.Key == key || actual.Rows[actual.Key] != actual.Key {
		t.Fatal("map-key pointer alias lost")
	}
	*actual.Key = 9
	if *key != 3 {
		t.Fatal("map key original mutated")
	}
}

func TestRuntimeCloneValueRejectsOpaqueAndOverlap(t *testing.T) {
	type privateMutable struct{ values []int }
	type renamedMutex sync.Mutex
	pair := &struct{ A, B int }{1, 2}
	backing := []int{1, 2, 3, 4}
	type otherMap map[string]int
	m := map[string]int{"a": 1}
	for _, test := range []struct {
		name    string
		value   any
		message string
	}{
		{"lock", sync.Mutex{}, "synchronization"}, {"atomic", atomic.Int64{}, "synchronization"},
		{"renamed lock", renamedMutex{}, "unexported mutable"},
		{"channel", make(chan int), "opaque"}, {"nil channel", (chan int)(nil), "opaque"}, {"function", func() {}, "opaque"}, {"unsafe", unsafe.Pointer(pair), "opaque"},
		{"nil function", (func())(nil), "opaque"},
		{"private mutable", privateMutable{values: []int{1}}, "unexported mutable"},
		{"interior pointer", struct {
			Parent any
			Field  *int
		}{pair, &pair.B}, "overlapping/interior"},
		{"interior first", struct {
			Field  *int
			Parent any
		}{&pair.A, pair}, "overlapping/interior"},
		{"offset slices", [][]int{backing[:2], backing[1:3]}, "overlapping/interior"},
		{"limited capacity alias", [][]int{backing[:2:2], backing[:2]}, "overlapping/interior"},
		{"slice element pointer", struct {
			Rows  []int
			First *int
		}{backing, &backing[0]}, "overlapping/interior"},
		{"map conversion alias", []any{m, otherMap(m)}, "differing concrete types"},
	} {
		t.Run(test.name, func(t *testing.T) {
			actual, err := (Runtime{}).CloneValue(test.value)
			if actual != nil || err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("actual=%v err=%v", actual, err)
			}
			if pair.A != 1 || pair.B != 2 || !reflect.DeepEqual(backing, []int{1, 2, 3, 4}) || m["a"] != 1 {
				t.Fatal("source mutated on failure")
			}
		})
	}
}

func TestRuntimeCloneValueConcurrent(t *testing.T) {
	source := &cloneEntity{Name: "immutable source", Labels: map[string][]string{"a": {"v"}}}
	for index := 0; index < 16; index++ {
		t.Run("clone", func(t *testing.T) {
			t.Parallel()
			value, err := (Runtime{}).CloneValue(source)
			if err != nil {
				t.Fatal(err)
			}
			value.(*cloneEntity).Labels["a"][0] = "local"
			if source.Labels["a"][0] != "v" {
				t.Fatal("source changed")
			}
		})
	}
}

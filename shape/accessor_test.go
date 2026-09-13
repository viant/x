package shape

import (
	"reflect"
	"sync"
	"testing"
)

func TestAccessorValues(t *testing.T) {
	type Embedded struct{ Value []string }
	type record struct {
		*Embedded
		hidden int
	}
	accessor, err := Linked(reflect.TypeOf(record{})).Accessor("Value")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		initial *Embedded
	}{{"nil_parent", nil}, {"existing_parent", &Embedded{Value: []string{"old"}}}} {
		t.Run(tc.name, func(t *testing.T) {
			value := &record{Embedded: tc.initial}
			if _, err := accessor.Get(value); err != nil {
				t.Fatal(err)
			}
			if err := accessor.Set(value, []string{"new"}); err != nil {
				t.Fatal(err)
			}
			if value.Embedded == nil || !reflect.DeepEqual(value.Value, []string{"new"}) {
				t.Fatal(value)
			}
			if err := accessor.Set(value, "wrong"); err == nil {
				t.Fatal("wrong type accepted")
			}
			if err := accessor.Set(value, nil); err != nil || value.Value != nil {
				t.Fatalf("clear=%v,%v", value, err)
			}
		})
	}
	if _, err := Linked(reflect.TypeOf(record{})).Accessor("hidden"); err == nil {
		t.Fatal("unexported field accepted")
	}
	for _, target := range []any{nil, (*record)(nil), &struct{}{}, record{}} {
		if err := accessor.Set(target, []string{"x"}); err == nil {
			t.Fatalf("invalid target accepted: %T", target)
		}
	}
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			value := &record{}
			if err := accessor.Set(value, []string{"concurrent"}); err != nil {
				t.Error(err)
			}
		}()
	}
	workers.Wait()
}

package shape

import (
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
)

type rewriteLeaf struct {
	Value string
	Count int
}
type rewriteRoot struct {
	Left  *rewriteLeaf
	Right []rewriteLeaf
	Plain string
}

func TestRewritePreservesEveryUnchangedNode(t *testing.T) {
	original := reflect.TypeOf(rewriteRoot{})
	plan, err := CompileRewrite(original)
	if err != nil {
		t.Fatal(err)
	}
	for _, callback := range []FieldRewriter{nil, func(string, reflect.Type, *FieldMetadata) (*FieldMetadata, error) { return nil, nil }, func(_ string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) { c := *f; return &c, nil }} {
		actual, err := plan.Rewrite(callback)
		if err != nil || actual != original {
			t.Fatalf("no-op identity: %v %v", actual, err)
		}
	}
	actual, err := plan.Rewrite(func(path string, parent reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
		if path != "Left" || f.Name != "Value" {
			return nil, nil
		}
		if parent != reflect.TypeOf(rewriteLeaf{}) {
			t.Fatal("parent not linked")
		}
		return &FieldMetadata{Name: f.Name, Tag: `json:"-"`}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if actual == original || actual.Field(0).Type == original.Field(0).Type {
		t.Fatal("changed ancestors retained")
	}
	if actual.Field(1).Type != original.Field(1).Type || actual.Field(2).Type != original.Field(2).Type {
		t.Fatal("unchanged sibling replaced")
	}
	if actual.Field(0).Type.Elem().Field(1).Type != reflect.TypeOf(int(0)) {
		t.Fatal("unchanged leaf replaced")
	}
	if original.Field(0).Type.Elem().Field(0).Tag != "" {
		t.Fatal("original mutated")
	}
}

type RewriteEmbedded struct {
	Secret string
	Public int
}
type rewriteEmbeddedRoot struct {
	*RewriteEmbedded
	Other string
}

func TestRewritePreservesEmbeddingAndJSONPromotion(t *testing.T) {
	plan, _ := CompileRewrite(reflect.TypeOf(rewriteEmbeddedRoot{}))
	actual, err := plan.Rewrite(func(path string, parent reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
		if path == "RewriteEmbedded" && f.Name == "Secret" {
			return &FieldMetadata{Name: f.Name, Tag: `json:"-"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	field := actual.Field(0)
	if !field.Anonymous || field.Name != "RewriteEmbedded" || field.Type.Kind() != reflect.Pointer {
		t.Fatal("embedding changed")
	}
	value := reflect.New(actual)
	bytes, err := json.Marshal(value.Interface())
	if err != nil || string(bytes) != `{"Other":""}` {
		t.Fatalf("nil embedded %s %v", bytes, err)
	}
	value.Elem().Field(0).Set(reflect.New(field.Type.Elem()))
	value.Elem().Field(0).Elem().FieldByName("Secret").SetString("hidden")
	if err = json.Unmarshal([]byte(`{"Secret":"attack","Public":7}`), value.Interface()); err != nil {
		t.Fatal(err)
	}
	bytes, err = json.Marshal(value.Interface())
	if err != nil || string(bytes) != `{"Public":7,"Other":""}` {
		t.Fatalf("promoted %s %v", bytes, err)
	}
	if value.Elem().Field(0).Elem().FieldByName("Secret").String() != "hidden" {
		t.Fatal("hidden field decoded")
	}
}

type rewriteRecursive struct {
	Next *rewriteRecursive
	Name string
}
type rewriteCustom struct{ Value string }

func (rewriteCustom) MarshalJSON() ([]byte, error) { return []byte(`"custom"`), nil }

type rewriteZero struct{ Value string }

func (rewriteZero) IsZero() bool { return true }
func TestRewriteRejectsUnrepresentableChanges(t *testing.T) {
	for _, original := range []reflect.Type{reflect.TypeOf(rewriteRecursive{}), reflect.TypeOf(rewriteCustom{}), reflect.TypeOf(rewriteZero{})} {
		plan, _ := CompileRewrite(original)
		actual, err := plan.Rewrite(func(string, reflect.Type, *FieldMetadata) (*FieldMetadata, error) { return nil, nil })
		if err != nil || actual != original {
			t.Fatal("opaque no-op", err)
		}
		_, err = plan.Rewrite(func(_ string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
			return &FieldMetadata{Name: f.Name, Tag: `json:"-"`}, nil
		})
		if err == nil {
			t.Fatal("expected projection error")
		}
	}
}

func TestRewriteErrorsRenamesAndConcurrency(t *testing.T) {
	plan, _ := CompileRewrite(reflect.TypeOf(rewriteLeaf{}))
	failure := errors.New("denied")
	if _, err := plan.Rewrite(func(string, reflect.Type, *FieldMetadata) (*FieldMetadata, error) { return nil, failure }); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	for _, name := range []string{"private", "Count", "bad-name"} {
		if _, err := plan.Rewrite(func(_ string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
			if f.Name == "Value" {
				return &FieldMetadata{Name: name, Tag: f.Tag}, nil
			}
			return nil, nil
		}); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	var wg sync.WaitGroup
	for n := 0; n < 16; n++ {
		wg.Add(1)
		go func(hide bool) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				actual, err := plan.Rewrite(func(_ string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
					if hide && f.Name == "Value" {
						return &FieldMetadata{Name: "Renamed", Tag: `json:"-"`}, nil
					}
					return nil, nil
				})
				if err != nil {
					t.Error(err)
					return
				}
				if !hide && actual != plan.Original() {
					t.Error("cross-caller mutation")
				}
			}
		}(n%2 == 0)
	}
	wg.Wait()
}

func TestRewriteContainersAndParentPaths(t *testing.T) {
	type root struct {
		Items  map[string][]*rewriteLeaf
		Stable [2]rewriteLeaf
	}
	plan, _ := CompileRewrite(reflect.TypeOf(root{}))
	actual, err := plan.Rewrite(func(path string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
		if path == "Items{}[]" && f.Name == "Value" {
			return &FieldMetadata{Name: f.Name, Tag: `json:"value"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if actual.Field(0).Type == plan.Original().Field(0).Type || actual.Field(1).Type != plan.Original().Field(1).Type {
		t.Fatal("container identity")
	}
}

func BenchmarkRewriteLinkedNoChange(b *testing.B) {
	plan, _ := CompileRewrite(reflect.TypeOf(rewriteRoot{}))
	callback := func(string, reflect.Type, *FieldMetadata) (*FieldMetadata, error) { return nil, nil }
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if actual, err := plan.Rewrite(callback); err != nil || actual != plan.Original() {
			b.Fatal(err)
		}
	}
}

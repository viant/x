package shape

import (
	"encoding/json"
	"reflect"
	"testing"
)

type projectionLeaf struct {
	Secret string
	Public int
}
type projectionRoot struct {
	First, Second *projectionLeaf
	Rows          []*projectionLeaf
	ByID          map[string]*projectionLeaf
	Stable        *rewriteLeaf
}

func TestProjectionRetaggedNestedValuesAndRoundTrip(t *testing.T) {
	original := reflect.TypeOf(&projectionRoot{})
	walk, _ := CompileRewrite(original)
	effective, err := walk.Rewrite(func(_ string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
		if f.Name == "Secret" {
			return &FieldMetadata{Name: "Hidden", Tag: `json:"-"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	forward, err := CompileProjection(original, effective)
	if err != nil {
		t.Fatal(err)
	}
	backward, err := CompileProjection(effective, original)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &projectionLeaf{Secret: "private", Public: 7}
	source := &projectionRoot{First: leaf, Second: leaf, Rows: []*projectionLeaf{leaf, nil}, ByID: map[string]*projectionLeaf{"x": leaf}, Stable: &rewriteLeaf{Value: "unchanged"}}
	projected, err := forward.Apply(source)
	if err != nil {
		t.Fatal(err)
	}
	v := reflect.ValueOf(projected).Elem()
	if v.Field(0).Pointer() != v.Field(1).Pointer() {
		t.Fatal("shared changed pointers split")
	}
	if v.FieldByName("Stable").Interface() != source.Stable {
		t.Fatal("unchanged linked subtree copied")
	}
	encoded, err := json.Marshal(projected)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(encoded, &doc); err != nil {
		t.Fatal(err)
	}
	if _, exists := doc["First"].(map[string]any)["Secret"]; exists {
		t.Fatal("secret leaked")
	}
	if _, exists := doc["First"].(map[string]any)["Hidden"]; exists {
		t.Fatal("hidden member leaked")
	}
	roundtrip, err := backward.Apply(projected)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(roundtrip, source) {
		t.Fatalf("round trip lost values: %+v", roundtrip)
	}
	if _, err = forward.Apply(&projectionLeaf{}); err == nil {
		t.Fatal("wrong source accepted")
	}
}

func TestProjectionEmbeddingNilAndPrivateStorage(t *testing.T) {
	type root struct {
		*RewriteEmbedded
		private int
	}
	original := reflect.TypeOf(&root{})
	walk, _ := CompileRewrite(original)
	effective, err := walk.Rewrite(func(path string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
		if path == "" && f.Name == "RewriteEmbedded" {
			return &FieldMetadata{Name: f.Name, Tag: `json:"-"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := CompileProjection(original, effective)
	if err != nil {
		t.Fatal(err)
	}
	value, err := plan.Apply(&root{private: 42})
	if err != nil {
		t.Fatal(err)
	}
	if reflect.ValueOf(value).Elem().Field(1).Int() != 42 || !reflect.ValueOf(value).Elem().Field(0).IsNil() {
		t.Fatal("private storage/nil changed")
	}
	back, _ := CompileProjection(effective, original)
	restored, err := back.Apply(value)
	if err != nil || restored.(*root).private != 42 {
		t.Fatal(err)
	}
}

func TestProjectionRejectsStorageChangesAndReusesOriginal(t *testing.T) {
	typ := reflect.TypeOf(&rewriteLeaf{})
	plan, err := CompileProjection(typ, typ)
	if err != nil {
		t.Fatal(err)
	}
	value := &rewriteLeaf{}
	actual, err := plan.Apply(value)
	if err != nil || actual != value {
		t.Fatal("identity", err)
	}
	if _, err = CompileProjection(reflect.TypeOf(int(0)), reflect.TypeOf(int64(0))); err == nil {
		t.Fatal("leaf conversion allowed")
	}
	if _, err = CompileProjection(reflect.TypeOf(map[string]int{}), reflect.TypeOf(map[int]int{})); err == nil {
		t.Fatal("key conversion allowed")
	}
}

func BenchmarkProjectionUnchanged(b *testing.B) {
	typ := reflect.TypeOf(&projectionRoot{})
	plan, _ := CompileProjection(typ, typ)
	value := &projectionRoot{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		actual, err := plan.Apply(value)
		if err != nil || actual != value {
			b.Fatal(err)
		}
	}
}

func BenchmarkProjectionNestedRename(b *testing.B) {
	typ := reflect.TypeOf(&projectionRoot{})
	walk, _ := CompileRewrite(typ)
	effective, err := walk.Rewrite(func(_ string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
		if f.Name == "Secret" {
			return &FieldMetadata{Name: "Hidden", Tag: `json:"-"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		b.Fatal(err)
	}
	plan, err := CompileProjection(typ, effective)
	if err != nil {
		b.Fatal(err)
	}
	leaf := &projectionLeaf{Secret: "private", Public: 7}
	value := &projectionRoot{First: leaf, Second: leaf, Rows: []*projectionLeaf{leaf}, ByID: map[string]*projectionLeaf{"one": leaf}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := plan.Apply(value); err != nil {
			b.Fatal(err)
		}
	}
}

package shape

import (
	"reflect"
	"strings"
	"sync"
	"testing"
)

type RepresentationEmbedded struct {
	Public string
	Secret string
}

func (*RepresentationEmbedded) Identity() string { return "linked" }

type representationEnvelope struct {
	*RepresentationEmbedded
	Value  int
	Stable *struct{ Keep string }
}

func TestRepresentationSupportsMethodBearingEmbedding(t *testing.T) {
	typ := reflect.TypeFor[representationEnvelope]()
	plan, err := CompileRewrite(typ)
	if err != nil {
		t.Fatal(err)
	}
	base, err := plan.RewriteRepresentation(nil)
	if err != nil {
		t.Fatal(err)
	}
	edited, err := plan.RewriteRepresentation(func(path string, parent reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
		if f.Name == "Secret" || f.Name == "Value" {
			f.Tag = `json:"-"`
			return f, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !edited.Changed() || edited.Original() != typ {
		t.Fatal("lost storage authority")
	}
	embedded := edited.Field(0)
	if !embedded.Anonymous || embedded.Type != reflect.TypeFor[*RepresentationEmbedded]() || embedded.Offset != typ.Field(0).Offset {
		t.Fatal("embedding changed")
	}
	if edited.FieldShape(0).Original() != base.FieldShape(0).Original() {
		t.Fatal("linked child replaced")
	}
	if edited.FieldShape(0).Elem().Field(1).Tag.Get("json") != "-" || edited.Field(1).Tag.Get("json") != "-" {
		t.Fatal("edits missing")
	}
	if edited.FieldShape(2) != base.FieldShape(2) {
		t.Fatal("unchanged subtree not reused")
	}
	if base.Field(1).Tag != "" || base.FieldShape(0).Elem().Field(1).Tag != "" {
		t.Fatal("base metadata mutated")
	}
	siblingOnly, err := plan.RewriteRepresentation(func(_ string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
		if f.Name == "Value" {
			return &FieldMetadata{Name: f.Name, Tag: `json:"-"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if siblingOnly.FieldShape(0) != base.FieldShape(0) {
		t.Fatal("unchanged embedded node lost identity")
	}
	if _, err = siblingOnly.Reflected(); err == nil {
		t.Fatal("reflection restriction unexpectedly absent")
	}
	if linked, err := base.Reflected(); err != nil || linked != typ {
		t.Fatal("no-op lost linked type", err)
	}
}
func TestRepresentationNoopAndCanonicalIndexIsolation(t *testing.T) {
	plan, _ := CompileRewrite(reflect.TypeFor[representationEnvelope]())
	base, _ := plan.RewriteRepresentation(nil)
	actual, err := plan.RewriteRepresentation(func(_ string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
		f.Tag = `json:"-"`
		return nil, nil
	})
	if err != nil || actual != base {
		t.Fatal("nil edit changed representation", err)
	}
	field := actual.Field(0)
	field.Index[0] = 99
	if actual.Field(0).Index[0] != 0 {
		t.Fatal("caller mutated cached field index")
	}
}
func TestRepresentationConcurrentVariants(t *testing.T) {
	plan, _ := CompileRewrite(reflect.TypeFor[representationEnvelope]())
	var group sync.WaitGroup
	for i := 0; i < 16; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			actual, err := plan.RewriteRepresentation(func(_ string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
				if f.Name == "Value" {
					return &FieldMetadata{Name: f.Name, Tag: `description:"caller"`}, nil
				}
				return nil, nil
			})
			if err != nil || actual.Field(1).Tag.Get("description") != "caller" {
				t.Errorf("variant: %v", err)
			}
		}()
	}
	group.Wait()
}
func TestRepresentationRejectsInvalidRename(t *testing.T) {
	plan, _ := CompileRewrite(reflect.TypeFor[representationEnvelope]())
	_, err := plan.RewriteRepresentation(func(_ string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
		if f.Name == "Value" {
			return &FieldMetadata{Name: "Stable"}, nil
		}
		return nil, nil
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatal(err)
	}
}

func TestRepresentationJSONFieldsUseEffectiveMetadataAndOriginalIndexes(t *testing.T) {
	plan, _ := CompileRewrite(reflect.TypeFor[representationEnvelope]())
	effective, err := plan.RewriteRepresentation(func(_ string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
		switch f.Name {
		case "Secret":
			return &FieldMetadata{Name: f.Name, Tag: `json:"-"`}, nil
		case "Value":
			return &FieldMetadata{Name: "Renamed", Tag: `json:"visible"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	fields, err := effective.JSONFields()
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 3 {
		t.Fatalf("selected fields %+v", fields)
	}
	for _, f := range fields {
		if f.Name == "Secret" {
			t.Fatal("hidden promoted field")
		}
		if f.Name == "Public" && (!reflect.DeepEqual(f.Field.Index, []int{0, 0}) || !f.OptionalHolder) {
			t.Fatal("embedded pointer ownership lost")
		}
		if f.Name == "visible" && (f.Field.Name != "Renamed" || !reflect.DeepEqual(f.Field.Index, []int{1}) || effective.FieldShapeByIndex(f.Field.Index).Original() != reflect.TypeFor[int]()) {
			t.Fatal("canonical storage mapping lost")
		}
	}
}

type RepresentationLeft struct{ *RepresentationEmbedded }
type RepresentationRight struct{ *RepresentationEmbedded }
type representationDiamond struct {
	RepresentationLeft
	RepresentationRight
}

func TestRepresentationJSONDominanceIsOccurrenceSpecific(t *testing.T) {
	plan, _ := CompileRewrite(reflect.TypeFor[representationDiamond]())
	base, _ := plan.RewriteRepresentation(nil)
	fields, err := base.JSONFields()
	if err != nil || len(fields) != 0 {
		t.Fatal("ambiguous original fields must not be published", fields, err)
	}
	variant, err := plan.RewriteRepresentation(func(path string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
		if strings.HasPrefix(path, "RepresentationLeft.") && f.Name == "Secret" {
			return &FieldMetadata{Name: f.Name, Tag: `json:"-"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	fields, err = variant.JSONFields()
	if err != nil || len(fields) != 1 || fields[0].Name != "Secret" || !reflect.DeepEqual(fields[0].Field.Index, []int{1, 0, 1}) {
		t.Fatalf("occurrence metadata was conflated: %+v %v", fields, err)
	}
}
func TestRepresentationRetainsOpaqueCodecBoundary(t *testing.T) {
	plan, _ := CompileRewrite(reflect.TypeFor[rewriteCustom]())
	base, err := plan.RewriteRepresentation(nil)
	if err != nil || base.Original() != reflect.TypeFor[rewriteCustom]() {
		t.Fatal(err)
	}
	_, err = plan.RewriteRepresentation(func(_ string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
		return &FieldMetadata{Name: f.Name, Tag: `json:"-"`}, nil
	})
	if err == nil {
		t.Fatal("custom JSON owner was rewritten")
	}
}

func TestRepresentationLookupUsesCanonicalContainerPaths(t *testing.T) {
	type container struct {
		Rows   []*representationEnvelope
		ByName map[string]*representationEnvelope
	}
	plan, _ := CompileRewrite(reflect.TypeFor[container]())
	effective, err := plan.RewriteRepresentation(func(_ string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
		if f.Name == "Value" {
			return &FieldMetadata{Name: "Renamed", Tag: `json:"visible"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"Rows.Value", "ByName.Value"} {
		item, err := effective.Lookup(path)
		if err != nil || item.Original() != reflect.TypeFor[int]() {
			t.Fatalf("%s: %v", path, err)
		}
	}
	if _, err := effective.Lookup("Rows.Renamed"); err == nil {
		t.Fatal("public name was treated as a canonical storage path")
	}
	again, err := plan.RewriteRepresentation(func(_ string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
		if f.Name == "Value" {
			return &FieldMetadata{Name: "Renamed", Tag: `json:"visible"`}, nil
		}
		return nil, nil
	})
	if err != nil || effective.Signature() != again.Signature() {
		t.Fatal("metadata signature is not stable", err)
	}
}

func TestMetadataRetainsIsZeroMethodAuthority(t *testing.T) {
	original := reflect.TypeFor[rewriteZero]()
	plan, _ := CompileRewrite(original)
	metadata, err := plan.RewriteRepresentation(func(_ string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
		return &FieldMetadata{Name: f.Name, Tag: `json:"-"`}, nil
	})
	if err != nil || metadata.Original() != original {
		t.Fatal("metadata lost original IsZero authority", err)
	}
	if _, err = metadata.Reflected(); err == nil {
		t.Fatal("reflected replacement would lose IsZero and must fail")
	}
}

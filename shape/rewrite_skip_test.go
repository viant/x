package shape

import (
	"reflect"
	"testing"
)

func TestRewriteSkipChildrenPreservesStorageAndSiblingTraversal(t *testing.T) {
	type leaf struct{ Value string }
	type root struct {
		Skip []*leaf
		Keep *leaf
	}
	plan, _ := CompileRewrite(reflect.TypeFor[root]())
	for _, metadata := range []bool{false, true} {
		visits := []string{}
		callback := func(path string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
			visits = append(visits, path+"."+f.Name)
			if f.Name == "Skip" {
				return &FieldMetadata{Name: f.Name, Tag: f.Tag, SkipChildren: true}, nil
			}
			return nil, nil
		}
		if metadata {
			before, _ := plan.RewriteRepresentation(nil)
			actual, err := plan.RewriteRepresentation(callback)
			if err != nil || actual != before {
				t.Fatal("skip-only changed representation", err)
			}
		} else {
			actual, err := plan.Rewrite(callback)
			if err != nil || actual != reflect.TypeFor[root]() {
				t.Fatal("skip-only changed reflected type", err)
			}
		}
		if !reflect.DeepEqual(visits, []string{".Skip", ".Keep", "Keep.Value"}) {
			t.Fatal(visits)
		}
	}
}
func TestRewriteSkipCanAccompanyTagEdit(t *testing.T) {
	type root struct{ Child *struct{ Value string } }
	plan, _ := CompileRewrite(reflect.TypeFor[root]())
	original, _ := plan.RewriteRepresentation(nil)
	actual, err := plan.RewriteRepresentation(func(path string, _ reflect.Type, f *FieldMetadata) (*FieldMetadata, error) {
		if path != "" {
			t.Fatal("descended despite skip")
		}
		return &FieldMetadata{Name: f.Name, Tag: `json:"-"`, SkipChildren: true}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if actual.Field(0).Tag.Get("json") != "-" || actual.FieldShape(0) != original.FieldShape(0) {
		t.Fatal("edit/linked child identity lost")
	}
}

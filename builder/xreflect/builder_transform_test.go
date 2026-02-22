package xreflect

import (
	"reflect"
	"testing"

	mdl "github.com/viant/x/syntetic/model"
	trf "github.com/viant/x/syntetic/model/transform"
)

type mapResolver map[string]reflect.Type

func (m mapResolver) Resolve(pkgPath, name string) (reflect.Type, bool) {
	t, ok := m[pkgPath+"."+name]
	return t, ok
}

func TestBuildNode_FromTags_TypeOverride(t *testing.T) {
	// Struct with a field tagged to override its type to test/pkg.Alias
	n := &mdl.Struct{Fields: []mdl.Field{{
		Name: "X",
		Tag:  `x:"type=test/pkg.Alias"`,
		Type: &mdl.Basic{Name: "int"},
	}}}

	// Resolver maps the named type to a concrete reflect.Type
	target := reflect.TypeOf(uint16(0))
	res := mapResolver{"test/pkg.Alias": target}

	b := New(WithResolver(res), WithTransforms(trf.FromTags()))
	rt, err := b.BuildNode(n)
	if err != nil {
		t.Fatalf("BuildNode error: %v", err)
	}
	if rt.Kind() != reflect.Struct {
		t.Fatalf("expected struct, got %v", rt)
	}
	f := rt.Field(0)
	if f.Type != target {
		t.Fatalf("type override failed: got %v, want %v", f.Type, target)
	}
}

func TestBuildNode_FromTags_OmitRenameAndTag(t *testing.T) {
	n := &mdl.Struct{Fields: []mdl.Field{
		{Name: "A", Tag: `x:"omit"`, Type: &mdl.Basic{Name: "int"}},
		{Name: "b", Tag: `x:"rename=Renamed,tag=json:\"x\""`, Type: &mdl.Basic{Name: "string"}},
	}}
	b := New(WithTransforms(trf.FromTags()))
	rt, err := b.BuildNode(n)
	if err != nil {
		t.Fatalf("BuildNode error: %v", err)
	}
	if rt.NumField() != 1 {
		t.Fatalf("expected 1 field after omit, got %d", rt.NumField())
	}
	f := rt.Field(0)
	if f.Name != "Renamed" {
		t.Fatalf("rename failed: got %s", f.Name)
	}
	if string(f.Tag) != "json:\"x\"" {
		t.Fatalf("tag override failed: got %q", string(f.Tag))
	}
}

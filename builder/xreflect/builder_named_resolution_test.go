package xreflect

import (
	"reflect"
	"strings"
	"testing"

	mdl "github.com/viant/x/syntetic/model"
)

type testInterfaceResolver struct {
	t reflect.Type
}

func (r testInterfaceResolver) ResolveInterface(methods []mdl.Method) (reflect.Type, bool) {
	if len(methods) == 1 && methods[0].Name == "Read" {
		return r.t, true
	}
	return nil, false
}

func TestBuildNode_UnresolvedNamed_TolerantFallback(t *testing.T) {
	b := New()
	rt, err := b.BuildNode(&mdl.Named{PkgPath: "acme/pkg", Name: "Order"})
	if err != nil {
		t.Fatalf("expected no error in tolerant mode, got: %v", err)
	}
	if rt == nil || rt.Kind() != reflect.Interface {
		t.Fatalf("expected interface{} fallback, got: %v", rt)
	}
}

func TestBuildNode_UnresolvedNamed_StrictError(t *testing.T) {
	b := New(WithStrictNamedResolution(true))
	_, err := b.BuildNode(&mdl.Named{PkgPath: "acme/pkg", Name: "Order"})
	if err == nil {
		t.Fatalf("expected error in strict mode")
	}
	if !strings.Contains(err.Error(), "unresolved named type acme/pkg.Order") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildNode_UnresolvedNamed_StrictErrorWithCandidates(t *testing.T) {
	cache := map[string]reflect.Type{
		"foo/one.Order": reflect.TypeOf(0),
		"foo/two.Order": reflect.TypeOf(""),
		"foo/one.User":  reflect.TypeOf(false),
	}
	b := New(WithCache(cache), WithStrictNamedResolution(true))
	_, err := b.BuildNode(&mdl.Named{PkgPath: "acme/pkg", Name: "Order"})
	if err == nil {
		t.Fatalf("expected error in strict mode")
	}
	msg := err.Error()
	if !strings.Contains(msg, "foo/one.Order") || !strings.Contains(msg, "foo/two.Order") {
		t.Fatalf("expected candidate hints, got: %v", msg)
	}
}

func TestBuildNode_UnresolvedNamed_Reporter(t *testing.T) {
	called := false
	var gotPkg string
	var gotName string
	var gotCandidates []string

	cache := map[string]reflect.Type{
		"foo/one.Order": reflect.TypeOf(0),
	}
	b := New(
		WithCache(cache),
		WithStrictNamedResolution(true),
		WithUnresolvedNamedReporter(func(pkgPath, name string, candidates []string) {
			called = true
			gotPkg = pkgPath
			gotName = name
			gotCandidates = candidates
		}),
	)

	_, _ = b.BuildNode(&mdl.Named{PkgPath: "acme/pkg", Name: "Order"})

	if !called {
		t.Fatalf("expected reporter callback")
	}
	if gotPkg != "acme/pkg" || gotName != "Order" {
		t.Fatalf("unexpected callback payload: pkg=%s name=%s", gotPkg, gotName)
	}
	if len(gotCandidates) != 1 || gotCandidates[0] != "foo/one.Order" {
		t.Fatalf("unexpected candidates: %#v", gotCandidates)
	}
}

func TestBuildNode_StrictNamed_WithUnderlyingPrecedence(t *testing.T) {
	b := New(WithStrictNamedResolution(true))
	n := &mdl.Named{
		PkgPath: "acme/pkg",
		Name:    "Order",
		Underlying: &mdl.Struct{
			Fields: []mdl.Field{
				{Name: "ID", Type: &mdl.Basic{Name: "int"}},
			},
		},
	}

	rt, err := b.BuildNode(n)
	if err != nil {
		t.Fatalf("expected underlying to materialize in strict mode, got: %v", err)
	}
	if rt == nil || rt.Kind() != reflect.Struct {
		t.Fatalf("expected struct type from underlying, got: %v", rt)
	}
	if rt.NumField() != 1 || rt.Field(0).Name != "ID" {
		t.Fatalf("unexpected underlying materialization: %v", rt)
	}
}

func TestBuildNode_AnonymousRecursive_DefaultError(t *testing.T) {
	root := &mdl.Struct{}
	root.Fields = []mdl.Field{
		{Name: "ID", Type: &mdl.Basic{Name: "int"}},
		{Name: "Next", Type: &mdl.Pointer{Elem: root}},
	}

	b := New()
	_, err := b.BuildNode(root)
	if err == nil {
		t.Fatalf("expected anonymous recursive struct error")
	}
	if !strings.Contains(err.Error(), "anonymous recursive struct not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildNode_AnonymousRecursive_Tolerated(t *testing.T) {
	root := &mdl.Struct{}
	root.Fields = []mdl.Field{
		{Name: "ID", Type: &mdl.Basic{Name: "int"}},
		{Name: "Next", Type: &mdl.Pointer{Elem: root}},
	}

	var names []string
	b := New(
		WithAllowAnonymousRecursion(true),
		WithAnonymousRecursionReporter(func(name string) { names = append(names, name) }),
	)
	rt, err := b.BuildNode(root)
	if err != nil {
		t.Fatalf("expected tolerant anonymous recursion build, got: %v", err)
	}
	if rt == nil || rt.Kind() != reflect.Struct {
		t.Fatalf("expected struct, got: %v", rt)
	}
	if rt.NumField() != 2 {
		t.Fatalf("unexpected field count: %d", rt.NumField())
	}
	nextField := rt.Field(1)
	if nextField.Name != "Next" {
		t.Fatalf("unexpected field name: %s", nextField.Name)
	}
	if nextField.Type.Kind() != reflect.Ptr || nextField.Type.Elem().Kind() != reflect.Interface {
		t.Fatalf("expected pointer to unknown fallback type, got: %v", nextField.Type)
	}
	if len(names) == 0 || !strings.HasPrefix(names[0], "AnonymousRecursive") {
		t.Fatalf("expected synthetic recursion name, got: %#v", names)
	}
}

func TestBuildNode_Interface_ResolvedByResolver(t *testing.T) {
	type Reader interface {
		Read([]byte) (int, error)
	}

	n := &mdl.Interface{
		Methods: []mdl.Method{
			{
				Name: "Read",
				Type: mdl.Func{
					Params: []mdl.Field{{Type: &mdl.Slice{Elem: &mdl.Basic{Name: "byte"}}}},
					Results: []mdl.Field{
						{Type: &mdl.Basic{Name: "int"}},
						{Type: &mdl.Basic{Name: "error"}},
					},
				},
			},
		},
	}

	want := reflect.TypeOf((*Reader)(nil)).Elem()
	b := New(WithInterfaceResolver(testInterfaceResolver{t: want}))
	rt, err := b.BuildNode(n)
	if err != nil {
		t.Fatalf("BuildNode error: %v", err)
	}
	if rt != want {
		t.Fatalf("unexpected interface type: got %v, want %v", rt, want)
	}
}

func TestBuildNode_Interface_StrictUnresolved(t *testing.T) {
	n := &mdl.Interface{
		Methods: []mdl.Method{
			{
				Name: "Apply",
				Type: mdl.Func{
					Params:  []mdl.Field{{Type: &mdl.Basic{Name: "int"}}},
					Results: []mdl.Field{{Type: &mdl.Basic{Name: "string"}}},
				},
			},
		},
	}

	b := New(WithStrictInterfaceResolution(true))
	_, err := b.BuildNode(n)
	if err == nil {
		t.Fatalf("expected unresolved interface error")
	}
	if !strings.Contains(err.Error(), "unresolved non-empty interface") || !strings.Contains(err.Error(), "Apply") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildNode_Interface_StrictUnresolved_Reporter(t *testing.T) {
	n := &mdl.Interface{
		Methods: []mdl.Method{
			{
				Name: "Apply",
				Type: mdl.Func{
					Params:  []mdl.Field{{Type: &mdl.Basic{Name: "int"}}},
					Results: []mdl.Field{{Type: &mdl.Basic{Name: "string"}}},
				},
			},
		},
	}

	called := false
	var got []mdl.Method
	b := New(
		WithStrictInterfaceResolution(true),
		WithUnresolvedInterfaceReporter(func(methods []mdl.Method) {
			called = true
			got = methods
		}),
	)
	_, _ = b.BuildNode(n)

	if !called {
		t.Fatalf("expected unresolved interface reporter callback")
	}
	if len(got) != 1 || got[0].Name != "Apply" {
		t.Fatalf("unexpected reporter payload: %#v", got)
	}
}

func TestBuildNode_Union_StrictUnresolved_Reporter(t *testing.T) {
	n := &mdl.Union{
		Terms: []mdl.Term{
			{Type: &mdl.Basic{Name: "int"}, Approx: true},
			{Type: &mdl.Basic{Name: "string"}},
		},
	}

	called := false
	var got []mdl.Term
	b := New(
		WithStrictUnionResolution(true),
		WithUnresolvedUnionReporter(func(terms []mdl.Term) {
			called = true
			got = terms
		}),
	)

	_, err := b.BuildNode(n)
	if err == nil {
		t.Fatalf("expected strict unresolved union error")
	}
	if !strings.Contains(err.Error(), "union/type-set") || !strings.Contains(err.Error(), "~int") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected unresolved union reporter callback")
	}
	if len(got) != 2 {
		t.Fatalf("unexpected terms: %#v", got)
	}
}

func TestBuildNode_Alias_TargetMaterialization_Reporter(t *testing.T) {
	alias := &mdl.Alias{
		PkgPath: "acme/pkg",
		Name:    "IDs",
		Target:  &mdl.Slice{Elem: &mdl.Basic{Name: "int"}},
	}

	called := false
	var gotAlias *mdl.Alias
	var gotType reflect.Type
	b := New(
		WithAliasReporter(func(a *mdl.Alias, resolved reflect.Type) {
			called = true
			gotAlias = a
			gotType = resolved
		}),
	)
	rt, err := b.BuildNode(alias)
	if err != nil {
		t.Fatalf("BuildNode error: %v", err)
	}
	if rt == nil || rt.Kind() != reflect.Slice || rt.Elem().Kind() != reflect.Int {
		t.Fatalf("unexpected alias target materialization: %v", rt)
	}
	if !called {
		t.Fatalf("expected alias reporter callback")
	}
	if gotAlias == nil || gotAlias.Name != "IDs" || gotAlias.PkgPath != "acme/pkg" {
		t.Fatalf("unexpected alias metadata: %#v", gotAlias)
	}
	if gotType != rt {
		t.Fatalf("unexpected resolved type in callback: got %v want %v", gotType, rt)
	}
}

func TestNormalizeUnionTerms(t *testing.T) {
	terms := []mdl.Term{
		{Type: &mdl.Basic{Name: "int"}, Approx: true},
		{Type: &mdl.Slice{Elem: &mdl.Basic{Name: "string"}}},
		{Type: &mdl.Named{PkgPath: "acme/pkg", Name: "Order"}},
	}

	got := NormalizeUnionTerms(terms)
	want := []string{"~int", "[]string", "acme/pkg.Order"}
	if len(got) != len(want) {
		t.Fatalf("unexpected length: got=%d want=%d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected normalized term at %d: got=%q want=%q", i, got[i], want[i])
		}
	}
}

func TestJoinUnionTerms(t *testing.T) {
	terms := []mdl.Term{
		{Type: &mdl.Basic{Name: "int"}, Approx: true},
		{Type: &mdl.Basic{Name: "string"}},
	}

	got := JoinUnionTerms(terms)
	want := "~int | string"
	if got != want {
		t.Fatalf("unexpected joined terms: got=%q want=%q", got, want)
	}
}

package shape

import (
	"github.com/viant/x"
	model "github.com/viant/x/syntetic/model"
	"reflect"
	"strings"
	"testing"
)

type mixedMethodLeaf struct{}

func (mixedMethodLeaf) Init(int) error     { return nil }
func (mixedMethodLeaf) Validate(int) error { return nil }

type mixedMethodInner struct{ mixedMethodLeaf }

func TestMixedPromotedMethodAuthority(t *testing.T) {
	source := `package hooks
type Wrapper struct{native.Inner}
type Other struct{}
func(Other)Init(string)error{return nil}
type Competing struct{native.Inner;Other}
type Accept[T interface{Init(int)error}]struct{}
type AcceptString[T interface{Init(string)error}]struct{}
type Scalar int
func(Scalar)Init(int)error{return nil}
type Slice []int
func(Slice)Validate(int)error{return nil}
`
	registry, _, _ := methodFixture(t, source)
	for _, descriptor := range registry {
		descriptor.SynteticType.Imports = map[string]*model.ImportRef{"native": {Path: "example.com/native"}}
	}
	inner := x.NewType(reflect.TypeOf(mixedMethodInner{}), x.WithName("Inner"), x.WithPkgPath("example.com/native"))
	registry[inner.Key()] = inner
	lookup := func(name string) (*x.Type, error) { return registry[name], nil }
	for _, name := range []string{"Wrapper", "Scalar", "Slice"} {
		actual, err := New(registry["example.com/hooks."+name], lookup).Methods(false)
		if err != nil || len(actual) == 0 {
			t.Fatalf("%s methods=%v error=%v", name, actual, err)
		}
	}
	if _, err := New(registry["example.com/hooks.Competing"], lookup).Methods(false); err == nil || !strings.Contains(err.Error(), "source method authority") {
		t.Fatalf("opaque competition error = %v", err)
	}
	resolver := Resolver{Package: "example.com/hooks", Lookup: lookup}
	if _, err := resolver.Resolve("Accept[Wrapper]"); err != nil {
		t.Fatalf("unambiguous mixed constraint: %v", err)
	}
	if _, err := resolver.Resolve("Accept[Competing]"); err == nil || !strings.Contains(err.Error(), "source method authority") {
		t.Fatalf("opaque method constraint error = %v", err)
	}
	native, _, _ := methodFixture(t, `package hooks
type Leaf struct{}
func(Leaf)Init(int)error{return nil}
func(Leaf)Validate(int)error{return nil}
type Inner struct{Leaf}`)
	for _, descriptor := range native {
		descriptor.PkgPath = "example.com/native"
		descriptor.SynteticType.PkgPath = "example.com/native"
		registry[descriptor.PkgPath+"."+descriptor.Name] = descriptor
	}
	registry["example.com/native.Inner"].Type = reflect.TypeOf(mixedMethodInner{})
	actual, err := New(registry["example.com/hooks.Competing"], lookup).Methods(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(actual) != 2 || actual[0].Name != "Init" || actual[0].Parameters[0] != "string" {
		t.Fatalf("source selection = %+v", actual)
	}
	if _, err := resolver.Resolve("AcceptString[Competing]"); err != nil {
		t.Fatalf("source-backed method constraint: %v", err)
	}
	if _, err := resolver.Resolve("Accept[Competing]"); err == nil {
		t.Fatal("wrong-depth int signature accepted")
	}
}

package x

import (
	"reflect"
	"testing"
)

func TestNewRegistry(t *testing.T) {

	type Foo struct {
		Id int
	}
	registry := NewRegistry()
	if registry == nil {
		t.Errorf("NewRegistry() = nil, want not nil")
	}
	registry.Register(NewType(reflect.TypeOf(Foo{})))
	fooType := registry.Lookup("github.com/viant/x.Foo")
	if fooType == nil {
		t.Fatal("failed to register type")
	}

}

package xreflect_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	builder "github.com/viant/x/builder/xreflect"
	loader "github.com/viant/x/loader/ast"
)

func TestLoadedInstantiationRespectsStrictResolution(t *testing.T) {
	pkg, err := loader.LoadPackageFS(context.Background(), fstest.MapFS{
		"go.mod": {Data: []byte("module example.com/p\n")},
		"p.go":   {Data: []byte("package p\ntype Box[T any] struct{}\nfunc F(value Box[int]) {}")},
	}, ".")
	if err != nil {
		t.Fatal(err)
	}
	_, err = builder.New(builder.WithStrictNamedResolution(true)).BuildNode(&pkg.Funcs[0].Type)
	if err == nil || !strings.Contains(err.Error(), "generic instantiation") {
		t.Fatalf("strict resolution: %v", err)
	}
	typ, err := builder.New().BuildNode(&pkg.Funcs[0].Type)
	if err != nil || typ != reflect.TypeOf(func(any) {}) {
		t.Fatalf("tolerant resolution: %v, %v", typ, err)
	}
}

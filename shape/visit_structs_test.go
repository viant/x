package shape

import (
	"errors"
	"reflect"
	"testing"
)

func TestVisitStructsCompositeOwnersAndEmptyNodes(t *testing.T) {
	type empty struct{}
	type order struct {
		Data *rewriteLeaf
		Fees map[int][]*empty
	}
	type campaign struct {
		Data   *rewriteLeaf
		Orders []*order
	}
	plan, err := CompileRewrite(reflect.TypeOf([]*campaign{}))
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	err = plan.VisitStructs(func(path string, typ reflect.Type) error {
		if typ.Kind() != reflect.Struct {
			t.Fatal(typ)
		}
		paths = append(paths, path)
		return nil
	})
	want := []string{"[]", "[].Data", "[].Orders[]", "[].Orders[].Data", "[].Orders[].Fees{}[]"}
	if err != nil || !reflect.DeepEqual(paths, want) {
		t.Fatal(paths, err)
	}
	failure := errors.New("stop")
	if err = plan.VisitStructs(func(string, reflect.Type) error { return failure }); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	recursive, _ := CompileRewrite(reflect.TypeOf(rewriteRecursive{}))
	count := 0
	if err = recursive.VisitStructs(func(string, reflect.Type) error { count++; return nil }); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}

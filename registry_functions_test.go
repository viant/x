package x

import (
	"errors"
	"github.com/viant/x/syntetic/model"
	"go/ast"
	"reflect"
	"strings"
	"testing"
)

func TestFunctionNativeInvocation(t *testing.T) {
	for _, test := range []struct {
		name     string
		function any
		args     []any
		want     []any
		fail     bool
	}{
		{"ordinary", func(n int) int { return n + 1 }, []any{2}, []any{3}, false},
		{"typed nil", func(n *int) bool { return n == nil }, []any{(*int)(nil)}, []any{true}, false},
		{"nil interface", func(n any) bool { return n == nil }, []any{nil}, []any{true}, false},
		{"variadic", func(prefix string, n ...any) int { return len(prefix) + len(n) }, []any{"a", 1, nil, (*int)(nil)}, []any{4}, false},
		{"zero variadic", func(n ...string) int { return len(n) }, nil, []any{0}, false},
		{"count mismatch", func(int) {}, nil, nil, true},
		{"type mismatch", func(int) {}, []any{"2"}, nil, true},
		{"nil scalar", func(int) {}, []any{nil}, nil, true},
		{"variadic mismatch", func(...string) {}, []any{2}, nil, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, err := NewFunction("example.com/app", "Function", test.function)
			if err != nil {
				t.Fatal(err)
			}
			if f.Type() != reflect.TypeOf(test.function) {
				t.Fatal("native signature changed")
			}
			actual, err := f.Call(test.args...)
			if (err != nil) != test.fail || !reflect.DeepEqual(actual, test.want) {
				t.Fatalf("actual=%#v error=%v", actual, err)
			}
		})
	}
}

func TestFunctionRegistrationValidation(t *testing.T) {
	for _, test := range []struct {
		pkg, name string
		value     any
	}{
		{"", "Factory", func() {}}, {".", "Factory", func() {}}, {"..", "Factory", func() {}}, {"/tmp/pkg", "Factory", func() {}}, {"example.com/app", "_", func() {}}, {"example.com/app", "wrong.name", func() {}}, {"example.com/app", "Factory", nil}, {"example.com/app", "Factory", (func())(nil)}, {"example.com/app", "Factory", 1},
	} {
		t.Run(test.pkg+test.name, func(t *testing.T) {
			if _, err := NewFunction(test.pkg, test.name, test.value); err == nil {
				t.Fatal("invalid export accepted")
			}
		})
	}
}

func TestFunctionPanicCause(t *testing.T) {
	cause := errors.New("factory failed")
	for _, value := range []any{cause, "string failure", nil} {
		f, err := NewFunction("example.com/app", "Factory", func() { panic(value) })
		if err != nil {
			t.Fatal(err)
		}
		results, err := f.Call()
		var failure *FunctionPanicError
		if results != nil || !errors.As(err, &failure) || failure.Key != f.Key() {
			t.Fatalf("results=%v error=%v", results, err)
		}
		if value == cause && !errors.Is(err, cause) {
			t.Fatal("panic error identity lost")
		}
	}
}

func TestRegistryFunctionsSnapshotAndAuthority(t *testing.T) {
	registry := NewRegistry()
	calls := 0
	first, _ := NewFunction("example.com/one", "Factory", func() string { calls++; return "one" })
	second, _ := NewFunction("example.com/two", "Factory", func() string { return "two" })
	if err := registry.RegisterFunctions(first, second); err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.LookupFunction("Factory"); ok {
		t.Fatal("unqualified lookup accepted")
	}
	if _, ok := registry.LookupFunction("example.com/other.Factory"); ok {
		t.Fatal("package authority ignored")
	}
	if err := registry.RegisterFunctions(first); err == nil {
		t.Fatal("duplicate accepted")
	}
	third, _ := NewFunction("example.com/one", "Other", func() {})
	if err := registry.RegisterFunctions(third, second); err == nil {
		t.Fatal("conflicting batch accepted")
	}
	if _, ok := registry.LookupFunction(third.Key()); ok {
		t.Fatal("failed registration partially published")
	}
	snapshot, err := (Cloner{}).Registry(registry)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("snapshot invoked factory")
	}
	if err := registry.RegisterFunctions(third); err != nil {
		t.Fatal(err)
	}
	if _, ok := snapshot.LookupFunction(third.Key()); ok {
		t.Fatal("new source export leaked into snapshot")
	}
	entries := snapshot.Functions()
	entries[0] = nil
	found, ok := snapshot.LookupFunction(first.Key())
	if !ok {
		t.Fatal("snapshot missing export")
	}
	result, err := found.Call()
	if err != nil || result[0] != "one" {
		t.Fatalf("result=%v err=%v", result, err)
	}
	merged := NewRegistry()
	merged.Merge(registry)
	if len(merged.Functions()) != 0 {
		t.Fatal("type-only Merge changed semantics")
	}
	if err := merged.RegisterFunctions(registry.Functions()...); err != nil {
		t.Fatal(err)
	}
	if len(merged.Functions()) != 3 {
		t.Fatal("explicit callable merge lost exports")
	}
}

func TestRegistryFunctionInvocationOutsideLocks(t *testing.T) {
	registry := NewRegistry()
	function, _ := NewFunction("example.com/app", "Factory", func() error {
		child, _ := NewFunction("example.com/app", "Child", func() {})
		return registry.RegisterFunctions(child)
	})
	if err := registry.RegisterFunctions(function); err != nil {
		t.Fatal(err)
	}
	found, _ := registry.LookupFunction(function.Key())
	result, err := found.Call()
	if err != nil || result[0] != nil {
		t.Fatalf("result=%v error=%v", result, err)
	}
}

func TestRegistryFunctionConcurrentSnapshots(t *testing.T) {
	registry := NewRegistry()
	for index := 0; index < 16; index++ {
		index := index
		t.Run(strings.Repeat("x", index+1), func(t *testing.T) {
			t.Parallel()
			function, _ := NewFunction("example.com/app", "F"+strings.Repeat("x", index+1), func() {})
			if err := registry.RegisterFunctions(function); err != nil {
				t.Fatal(err)
			}
			snapshot, err := (Cloner{}).Registry(registry)
			if err != nil {
				t.Fatal(err)
			}
			for _, export := range snapshot.Functions() {
				if _, err := export.Call(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestRegistrySnapshotTypesAndFunctionsShareGeneration(t *testing.T) {
	listeners := 0
	registry := NewRegistry(WithRegistryScn(12), WithListener(func(*Type) { listeners++ }))
	source := &Type{Name: "Entity", PkgPath: "example.com/app", Type: reflect.TypeOf(struct{ ID int }{}), SynteticType: &model.Type{Name: "Entity", TypeSpec: &ast.TypeSpec{Name: ast.NewIdent("Entity"), Type: ast.NewIdent("int")}}}
	registry.Register(source)
	function, _ := NewFunction("example.com/app", "Factory", func() int { return 12 })
	if err := registry.RegisterFunctions(function); err != nil {
		t.Fatal(err)
	}
	snapshot, err := (Cloner{}).Registry(registry)
	if err != nil {
		t.Fatal(err)
	}
	if listeners != 1 || snapshot.Scn() != 12 {
		t.Fatalf("listeners=%d scn=%d", listeners, snapshot.Scn())
	}
	cloned := snapshot.Lookup(source.Key())
	if cloned == source || cloned.Type != source.Type || cloned.Scn != source.Scn || cloned.SynteticType.TypeSpec == source.SynteticType.TypeSpec {
		t.Fatal("type identity/AST snapshot mismatch")
	}
	cloned.SynteticType.TypeSpec.Type = ast.NewIdent("string")
	if source.SynteticType.Body() != "int" {
		t.Fatal("synthetic source mutated")
	}
	registry.Register(NewType(reflect.TypeOf(true), WithName("Later"), WithPkgPath("example.com/app")))
	other, _ := NewFunction("example.com/app", "LaterFactory", func() {})
	if err := registry.RegisterFunctions(other); err != nil {
		t.Fatal(err)
	}
	if snapshot.Lookup("example.com/app.Later") != nil {
		t.Fatal("later type leaked")
	}
	if _, ok := snapshot.LookupFunction(other.Key()); ok {
		t.Fatal("later function leaked")
	}
	got, _ := snapshot.LookupFunction(function.Key())
	result, err := got.Call()
	if err != nil || result[0] != 12 {
		t.Fatalf("snapshot callable=%v err=%v", result, err)
	}
	snapshot.Register(NewType(reflect.TypeOf(false), WithName("SnapshotOnly"), WithPkgPath("example.com/app")))
	if listeners != 2 {
		t.Fatal("snapshot retained source listener")
	}
}

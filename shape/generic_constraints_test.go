package shape

import (
	"github.com/viant/x"
	"go/ast"
	"go/types"
	"reflect"
	"strings"
	"testing"
)

const constraintSource = `package hooks
type Cmp[T comparable]struct{}
type Numeric[T interface{~int|~string}]struct{}
type Exact[T int]struct{}
type Sized[T interface{Size()int}]struct{}
type Dependent[S ~[]E,E any]struct{}
type Identifier int
func(Identifier)Size()int{return 0}
type PointerSized struct{}
func(*PointerSized)Size()int{return 0}
type NonComparable struct{Values []int}
type Other[E any]interface{Value()E}
type GetterString struct{}
func(GetterString)Value()string{return ""}
type GetterInt struct{}
func(GetterInt)Value()int{return 0}
type Holder[T Other[E],E any]struct{}
`

func TestGenericConstraintsMatchGoSatisfaction(t *testing.T) {
	registry, file, set := methodFixture(t, constraintSource)
	pkg, err := (&types.Config{}).Check("example.com/hooks", set, []*ast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	resolver := Resolver{Package: "example.com/hooks", Lookup: func(name string) (*x.Type, error) { return registry[name], nil }}
	for _, test := range []struct {
		generic   string
		arguments []string
	}{
		{"Cmp", []string{"int"}}, {"Cmp", []string{"any"}}, {"Cmp", []string{"map[string]int"}}, {"Cmp", []string{"NonComparable"}}, {"Cmp", []string{"*NonComparable"}},
		{"Numeric", []string{"Identifier"}}, {"Numeric", []string{"string"}}, {"Numeric", []string{"float64"}},
		{"Exact", []string{"int"}}, {"Exact", []string{"Identifier"}},
		{"Sized", []string{"Identifier"}}, {"Sized", []string{"PointerSized"}}, {"Sized", []string{"*PointerSized"}}, {"Sized", []string{"int"}},
		{"Dependent", []string{"[]string", "string"}}, {"Dependent", []string{"[]int", "string"}},
		{"Holder", []string{"GetterString", "string"}}, {"Holder", []string{"GetterInt", "string"}},
	} {
		expression := test.generic + "["
		arguments := []types.Type{}
		for i, arg := range test.arguments {
			if i > 0 {
				expression += ","
			}
			expression += arg
			value, err := types.Eval(set, pkg, 0, arg)
			if err != nil {
				t.Fatal(err)
			}
			arguments = append(arguments, value.Type)
		}
		expression += "]"
		t.Run(expression, func(t *testing.T) {
			_, compilerErr := types.Instantiate(nil, pkg.Scope().Lookup(test.generic).Type(), arguments, true)
			_, err := resolver.Resolve(expression)
			if (err != nil) != (compilerErr != nil) {
				t.Fatalf("Resolve error = %v, Go compiler = %v", err, compilerErr)
			}
		})
	}
}

func TestGenericConstraintAliasCycleFailsClosed(t *testing.T) {
	registry, _, _ := methodFixture(t, `package hooks
type Check[T comparable]struct{}
type AliasA = AliasB
type AliasB = AliasA
type Node struct{Next *Node}`)
	resolver := Resolver{Package: "example.com/hooks", Lookup: func(name string) (*x.Type, error) { return registry[name], nil }}
	if _, err := resolver.Resolve("Check[AliasA]"); err == nil || !strings.Contains(err.Error(), "cyclic alias") {
		t.Fatalf("alias cycle error = %v", err)
	}
	if _, err := resolver.Resolve("Check[Node]"); err != nil {
		t.Fatalf("valid named pointer cycle: %v", err)
	}
}

type linkedConstraintID int

func (linkedConstraintID) Size() int { return 0 }

func TestGenericConstraintsLinkedArgument(t *testing.T) {
	registry, _, _ := methodFixture(t, constraintSource)
	linked := x.NewType(reflect.TypeOf(linkedConstraintID(0)))
	registry[linked.Key()] = linked
	resolver := Resolver{Package: "example.com/hooks", Lookup: func(name string) (*x.Type, error) { return registry[name], nil }}
	for _, name := range []string{"Cmp", "Numeric", "Sized"} {
		if _, err := resolver.Resolve(name + "[" + linked.Key() + "]"); err != nil {
			t.Fatalf("%s linked constraint: %v", name, err)
		}
	}
}

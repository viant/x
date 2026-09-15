package shape

import (
	"github.com/viant/x"
	"testing"
)

func TestResolverComparableNamedTypes(t *testing.T) {
	registry, _, _ := methodFixture(t, `package hooks
type ID int
type Names []string
type Counts map[string]int
type Alias = Names
type Pair struct{ID ID;Name string}
type Dynamic any
type DynamicPair struct{Value any}
type Recursive struct{Next *Recursive}
`)
	resolver := Resolver{Package: "example.com/hooks", Lookup: func(name string) (*x.Type, error) { return registry[name], nil }}
	for _, tc := range []struct {
		source string
		want   bool
	}{{"ID", true}, {"Names", false}, {"Counts", false}, {"Alias", false}, {"Pair", true}, {"Dynamic", false}, {"DynamicPair", false}, {"*Names", true}, {"Recursive", true}, {"[2]ID", true}, {"[]int", false}, {"map[string]int", false}, {"any", false}} {
		t.Run(tc.source, func(t *testing.T) {
			got, err := resolver.IsComparable(tc.source)
			if err != nil || got != tc.want {
				t.Fatalf("got %v, %v; want %v", got, err, tc.want)
			}
		})
	}
	if _, err := resolver.IsComparable("Missing"); err == nil {
		t.Fatal("unresolved authority accepted")
	}
}

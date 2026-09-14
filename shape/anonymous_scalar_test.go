package shape

import (
	"go/ast"
	"go/parser"
	"reflect"
	"testing"

	"github.com/viant/x"
	"github.com/viant/x/syntetic/model"
)

func TestSyntheticAnonymousScalarKeepsFieldIndexes(t *testing.T) {
	for _, pointer := range []string{"", "*"} {
		t.Run("embedded="+pointer+"Amount", func(t *testing.T) {
			types := map[string]*x.Type{}
			for name, source := range map[string]string{
				"BaseAmount": "float64", "Amount": "BaseAmount", "Dimensions": "struct { Region string }",
				"Row": "struct { " + pointer + "Amount; " + pointer + "Dimensions; Next int }",
			} {
				expression, err := parser.ParseExpr(source)
				if err != nil {
					t.Fatal(err)
				}
				types["example.com/model."+name] = &x.Type{PkgPath: "example.com/model", Name: name, SynteticType: &model.Type{PkgPath: "example.com/model", Name: name, TypeSpec: &ast.TypeSpec{Name: ast.NewIdent(name), Type: expression}}}
			}
			shape := New(types["example.com/model.Row"], func(name string) (*x.Type, error) { return types[name], nil })
			fields, err := shape.Fields()
			if err != nil {
				t.Fatal(err)
			}
			if len(fields) != 4 {
				t.Fatalf("fields=%+v", fields)
			}
			for i, want := range []struct {
				name  string
				index []int
			}{{"Amount", []int{0}}, {"Dimensions", []int{1}}, {"Region", []int{1, 0}}, {"Next", []int{2}}} {
				if fields[i].Name != want.name || !reflect.DeepEqual(fields[i].Index, want.index) {
					t.Fatalf("field %d=%+v want=%+v", i, fields[i], want)
				}
			}
			identity, err := fields[0].CanonicalType()
			if err != nil || identity != pointer+"example.com/model.Amount" {
				t.Fatalf("scalar identity=%s err=%v", identity, err)
			}
		})
	}
}

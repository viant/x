package xtype

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

func TestStructTagLiteralRoundtrip(t *testing.T) {
	tags := []string{
		`json:"name" sql:"NAME"`,
		"sql:\"SELECT * FROM `project.dataset.table`\" json:\"name\"",
		"sql:\"`table`\\\\column\\\"quoted\\\"\"",
	}
	for _, tag := range tags {
		t.Run(tag, func(t *testing.T) {
			expression, err := ToAST(&Struct{Fields: []Field{{Name: "Value", Type: &Basic{Name: "string"}, Tag: tag}}}, NewImportSet("example"))
			if err != nil {
				t.Fatal(err)
			}
			var source bytes.Buffer
			if err := printer.Fprint(&source, token.NewFileSet(), expression); err != nil {
				t.Fatal(err)
			}
			parsed, err := parser.ParseExpr(source.String())
			if err != nil {
				t.Fatalf("invalid Go type: %v\n%s", err, source.String())
			}
			literal := parsed.(*ast.StructType).Fields.List[0].Tag.Value
			delimiter := byte('`')
			if strings.Contains(tag, "`") {
				delimiter = '"'
			}
			if literal[0] != delimiter {
				t.Fatalf("unexpected delimiter: %s", literal)
			}
			actual, err := strconv.Unquote(literal)
			if err != nil || actual != tag {
				t.Fatalf("tag changed: %q want %q: %v", actual, tag, err)
			}
		})
	}
}

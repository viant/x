package ast

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"testing"
)

func TestLoadPackageFS_Interfaces_Embedded(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/iface\n\n",
		"root/p/i.go": `package p

type Reader interface{ Read([]byte) (int, error) }
type Closer interface{ Close() error }
type ReadCloser interface{ Reader; Closer }
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	for _, n := range []string{"Reader", "Closer", "ReadCloser"} {
		if !pkg.HasType(n) {
			t.Fatalf("missing %s", n)
		}
	}
	rc := findTypeG(pkg, "ReadCloser")
	// Ensure interface body prints the embedded names; AST normalization hides ordering differences
	assertIfaceBody(t, rc.Body(), "interface{ Reader; Closer }")
}

func assertIfaceBody(t *testing.T, rendered, expected string) {
	t.Helper()
	parse := func(expr string) ast.Expr { n, _ := parser.ParseExpr(expr); return n.(ast.Expr) }
	var fs token.FileSet
	var a, b bytes.Buffer
	_ = printer.Fprint(&a, &fs, parse(rendered))
	_ = printer.Fprint(&b, &fs, parse(expected))
	if a.String() != b.String() {
		t.Fatalf("iface mismatch:\n got: %s\nwant: %s", a.String(), b.String())
	}
}

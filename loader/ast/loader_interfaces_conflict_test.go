package ast

import (
	"context"
	"testing"
)

// Behavior: allow embedded interface conflicts (no dedupe/flatten); we trust source AST.
func TestInterfaces_EmbeddedConflict_Allowed(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/ic\n\n",
		"root/p/i.go": `package p

type A interface{ M() }
type B interface{ M() }
type C interface{ A; B }
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	if !pkg.HasType("C") {
		t.Fatalf("expected interface C")
	}
	c := findTypeG(pkg, "C")
	// We accept the source form with embedded A and B and do not deduplicate methods.
	assertIfaceBody(t, c.Body(), "interface{ A; B }")
}

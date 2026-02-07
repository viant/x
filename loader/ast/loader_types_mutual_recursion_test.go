package ast

import (
	"context"
	"testing"
)

func TestLoadPackageFS_Types_MutualRecursion(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/mrec\n\n",
		"root/p/types.go": `package p

type A struct{ B *B }
type B struct{ A *A }
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	if !pkg.HasType("A") || !pkg.HasType("B") {
		t.Fatalf("missing types A or B")
	}
	bodies := map[string]string{}
	for _, tpe := range pkg.Types {
		bodies[tpe.Name] = tpe.Body()
	}
	if got := bodies["A"]; got != "struct{ B *B }" {
		t.Fatalf("A body mismatch: %s", got)
	}
	if got := bodies["B"]; got != "struct{ A *A }" {
		t.Fatalf("B body mismatch: %s", got)
	}
}

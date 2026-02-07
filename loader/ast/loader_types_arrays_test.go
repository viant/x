package ast

import (
	"context"
	"testing"
)

func TestLoadPackageFS_Types_Arrays(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/types\n\n",
		"root/p/arrays.go": `package p

type A1 [3]int
type A2 [2][3]int
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	for _, n := range []string{"A1", "A2"} {
		if !pkg.HasType(n) {
			t.Fatalf("missing type %s", n)
		}
	}
}

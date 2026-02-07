package ast

import (
	"context"
	"testing"
)

func TestLoadPackageFS_Types_Slices(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/types\n\n",
		"root/p/slices.go": `package p

type L1 []int
type T struct{ X int }
type L2 [][]*T
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	for _, n := range []string{"L1", "L2", "T"} {
		if !pkg.HasType(n) {
			t.Fatalf("missing type %s", n)
		}
	}
}

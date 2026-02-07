package ast

import (
	"context"
	"testing"
)

func TestLoadPackageFS_Types_Maps(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/types\n\n",
		"root/p/maps.go": `package p

type M1 map[string]int
type T struct{ X int }
type M2 map[string]map[string]*T
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	for _, n := range []string{"M1", "M2", "T"} {
		if !pkg.HasType(n) {
			t.Fatalf("missing type %s", n)
		}
	}
}

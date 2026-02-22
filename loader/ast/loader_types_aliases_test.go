package ast

import (
	"context"
	"testing"
)

func TestLoadPackageFS_Types_Aliases(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/types\n\n",
		"root/p/aliases.go": `package p

type MyInt = int
type T struct{ X int }
type TT = T
type MyMap = map[string]int
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	for _, n := range []string{"MyInt", "TT", "MyMap", "T"} {
		if !pkg.HasType(n) {
			t.Fatalf("missing type %s", n)
		}
	}
}

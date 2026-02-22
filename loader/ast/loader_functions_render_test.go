package ast

import (
	"context"
	"testing"
)

func TestPackage_RenderFilesWithMethods_FreeFunctions(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/fn\n\n",
		"root/p/a.go": `package p

type T struct{}
func (t T) M() {}
func F() int { return 1 }
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	files, err := pkg.RenderFilesWithMethods()
	if err != nil {
		t.Fatalf("RenderFilesWithMethods error: %v", err)
	}
	src := files["a.go"]
	if src == "" || !contains(src, "func F() int") {
		t.Fatalf("expected free function stub in output:\n%s", src)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(s) > len(sub) && indexOf(s, sub) >= 0))
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

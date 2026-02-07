package ast

import (
	"context"
	"testing"
)

func TestLoadPackageFS_Types_RecursiveNested(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/rec\n\n",
		"root/p/types.go": `package p

type Node struct {
    Children []*Node
    Index    map[string]*Node
    Next     *Node
}
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}
	if !pkg.HasType("Node") {
		t.Fatalf("missing type Node")
	}
	// Verify the rendered body is stable and expresses nested self-refs
	var body string
	for _, tpe := range pkg.Types {
		if tpe.Name == "Node" {
			body = tpe.Body()
		}
	}
	if body == "" || body[0:6] != "struct" {
		t.Fatalf("Node body unexpected prefix: %q", body)
	}
	if !(containsAll2(norm(body), "Children[]*Node", "Indexmap[string]*Node", "Next*Node")) {
		t.Fatalf("Node body missing fields: %s", body)
	}
}

func containsAll2(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func norm(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\n' || c == '\t' || c == '\r' {
			continue
		}
		out = append(out, c)
	}
	return string(out)
}

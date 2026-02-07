package ast

import (
	"context"
	"testing"
)

// Tests alias types that reference recursive defined types via pointer
// indirection. For example, a self-referential defined type P *P and
// aliases to containers of P (slices, maps). This is a valid pattern
// and should round-trip in Body() rendering.
func TestLoadPackageFS_Types_AliasRecursiveContainers(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/aliasrec\n\n",
		"root/p/types.go": `package p

// Pointer-recursive defined type
type P *P

// Aliases to containers of P
type List = []*P
type Graph = map[*P][]*P

// Struct-recursive defined type
type Node struct{ Next *Node }

// Aliases to containers of Node
type NodeList = []*Node
type NodeGraph = map[*Node][]*Node
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}

	// Presence
	for _, n := range []string{"P", "List", "Graph", "Node", "NodeList", "NodeGraph"} {
		if !pkg.HasType(n) {
			t.Fatalf("missing type %s", n)
		}
	}

	bodies := map[string]string{}
	for _, tpe := range pkg.Types {
		bodies[tpe.Name] = tpe.Body()
	}

	// Bodies for aliases should render underlying RHS expression
	if got := bodies["List"]; got != "[]*P" {
		t.Fatalf("List body mismatch: %s", got)
	}
	if got := bodies["Graph"]; got != "map[*P][]*P" {
		t.Fatalf("Graph body mismatch: %s", got)
	}
	if got := bodies["NodeList"]; got != "[]*Node" {
		t.Fatalf("NodeList body mismatch: %s", got)
	}
	if got := bodies["NodeGraph"]; got != "map[*Node][]*Node" {
		t.Fatalf("NodeGraph body mismatch: %s", got)
	}
}

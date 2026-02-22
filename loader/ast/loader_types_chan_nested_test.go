package ast

import (
	"context"
	"testing"
)

func TestLoadPackageFS_Types_ChansAndNested(t *testing.T) {
	fsys := MkFS(t, map[string]string{
		"root/go.mod": "module example.com/types\n\n",
		"root/p/types.go": `package p

// Nested composites and chan directions
type A map[string][]map[int][3]uint8
type SendOnly chan<- int
type RecvOnly <-chan string
type Both chan bool

// Function shapes (signatures only captured for free functions; here we use types)
type Fn1 func(int) string
type Fn2 func(prefix string, values ...int) (int, error)
type Fn3 func(chan<- int) (<-chan string, error)
`,
	})
	ctx := context.Background()
	pkg, err := LoadPackageFS(ctx, fsys, "root/p")
	if err != nil {
		t.Fatalf("LoadPackageFS error: %v", err)
	}

	// Quick presence checks
	for _, n := range []string{"A", "SendOnly", "RecvOnly", "Both", "Fn1", "Fn2", "Fn3"} {
		if !pkg.HasType(n) {
			t.Fatalf("missing type %s", n)
		}
	}

	// Spot-check rendered bodies for key shapes
	bodies := map[string]string{}
	for _, tpe := range pkg.Types {
		bodies[tpe.Name] = tpe.Body()
	}

	if got := bodies["A"]; got != "map[string][]map[int][3]uint8" {
		t.Fatalf("A body mismatch: %s", got)
	}
	if got := bodies["SendOnly"]; got != "chan<- int" {
		t.Fatalf("SendOnly body mismatch: %s", got)
	}
	if got := bodies["RecvOnly"]; got != "<-chan string" {
		t.Fatalf("RecvOnly body mismatch: %s", got)
	}
	if got := bodies["Both"]; got != "chan bool" {
		t.Fatalf("Both body mismatch: %s", got)
	}
	if got := bodies["Fn1"]; got != "func(int) string" {
		t.Fatalf("Fn1 body mismatch: %s", got)
	}
	if got := bodies["Fn2"]; got != "func(prefix string, values ...int) (int, error)" {
		t.Fatalf("Fn2 body mismatch: %s", got)
	}
	if got := bodies["Fn3"]; got != "func(chan<- int) (<-chan string, error)" {
		t.Fatalf("Fn3 body mismatch: %s", got)
	}
}

package shape

import (
	"reflect"
	"testing"
)

func TestRewritePlanSharesRepeatedAcyclicTypes(t *testing.T) {
	typ := reflect.TypeOf(struct{ Value string }{})
	const depth = 10
	for i := 0; i < depth; i++ {
		child := reflect.PointerTo(typ)
		typ = reflect.StructOf([]reflect.StructField{{Name: "Left", Type: child}, {Name: "Right", Type: child}})
	}
	plan, err := CompileRewrite(typ)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[*rewriteNode]bool{}
	var visit func(*rewriteNode)
	visit = func(n *rewriteNode) {
		if n == nil || seen[n] {
			return
		}
		seen[n] = true
		visit(n.elem)
		for _, field := range n.fields {
			visit(field.child)
		}
	}
	visit(plan.root)
	if len(seen) > 2*depth+2 {
		t.Fatalf("%d unique types expanded into %d retained nodes", 2*depth+2, len(seen))
	}
	metadata, err := plan.RewriteRepresentation(nil)
	if err != nil {
		t.Fatal(err)
	}
	seenMetadata := map[*Representation]bool{}
	var inspect func(*Representation)
	inspect = func(r *Representation) {
		if r == nil || seenMetadata[r] {
			return
		}
		seenMetadata[r] = true
		inspect(r.Elem())
		for i := 0; i < r.NumField(); i++ {
			inspect(r.FieldShape(i))
		}
	}
	inspect(metadata)
	if len(seenMetadata) > 2*depth+2 {
		t.Fatalf("shared plan expanded into %d representations", len(seenMetadata))
	}
}

func TestSharedMetadataKeepsPathSpecificEdits(t *testing.T) {
	type child struct {
		Value string `json:"value"`
	}
	type parent struct {
		Left  child
		Right child
	}
	plan, err := CompileRewrite(reflect.TypeOf(parent{}))
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := plan.RewriteRepresentation(func(path string, _ reflect.Type, field *FieldMetadata) (*FieldMetadata, error) {
		if path == "Right" && field.Name == "Value" {
			return &FieldMetadata{Name: field.Name, Tag: `json:"restricted"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata.FieldShape(0).Field(0).Tag.Get("json"); got != "value" {
		t.Fatalf("left policy changed: %s", got)
	}
	if got := metadata.FieldShape(1).Field(0).Tag.Get("json"); got != "restricted" {
		t.Fatalf("right policy lost: %s", got)
	}
	original, err := plan.RewriteRepresentation(nil)
	if err != nil {
		t.Fatal(err)
	}
	if original.FieldShape(1).Field(0).Tag.Get("json") != "value" {
		t.Fatal("policy mutated shared metadata")
	}
	reflected, err := plan.Rewrite(func(path string, _ reflect.Type, field *FieldMetadata) (*FieldMetadata, error) {
		if path == "Right" && field.Name == "Value" {
			return &FieldMetadata{Name: field.Name, Tag: `json:"restricted"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if reflected.Field(0).Type.Field(0).Tag.Get("json") != "value" || reflected.Field(1).Type.Field(0).Tag.Get("json") != "restricted" {
		t.Fatal("reflected rewrite mixed path-specific decisions")
	}
}

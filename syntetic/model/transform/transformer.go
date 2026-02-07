package transform

import (
	mdl "github.com/viant/x/syntetic/model"
)

// Transformer applies structural rewrites to model graphs. Implementations may
// operate in-place or return new values. Callers should not rely on mutation
// semantics; always use the returned value.
type Transformer interface {
	// ApplyPackage applies this transform to a package. Implementations may
	// walk files, types, and fields as needed.
	ApplyPackage(p *mdl.Package) *mdl.Package
	// ApplyType applies this transform to a single type.
	ApplyType(t *mdl.Type) *mdl.Type
	// ApplyNode applies this transform to a node.
	ApplyNode(n mdl.Node) mdl.Node
}

// Nop is a no-op transformer that returns inputs unchanged.
type Nop struct{}

func (Nop) ApplyPackage(p *mdl.Package) *mdl.Package { return p }
func (Nop) ApplyType(t *mdl.Type) *mdl.Type          { return t }
func (Nop) ApplyNode(n mdl.Node) mdl.Node            { return n }

// Compose builds a transformer that applies the provided transformers in order.
// Nil transformers are ignored. When no transformers are supplied, a no-op
// transformer is returned.
func Compose(ts ...Transformer) Transformer {
	var list []Transformer
	for _, t := range ts {
		if t != nil {
			list = append(list, t)
		}
	}
	if len(list) == 0 {
		return Nop{}
	}
	return chain{list: list}
}

type chain struct{ list []Transformer }

func (c chain) ApplyPackage(p *mdl.Package) *mdl.Package {
	out := p
	for _, t := range c.list {
		out = t.ApplyPackage(out)
	}
	return out
}
func (c chain) ApplyType(t *mdl.Type) *mdl.Type {
	out := t
	for _, tr := range c.list {
		out = tr.ApplyType(out)
	}
	return out
}
func (c chain) ApplyNode(n mdl.Node) mdl.Node {
	out := n
	for _, tr := range c.list {
		out = tr.ApplyNode(out)
	}
	return out
}

// Convenience helpers

// ApplyPackage applies transformers to a package.
func ApplyPackage(p *mdl.Package, ts ...Transformer) *mdl.Package {
	return Compose(ts...).ApplyPackage(p)
}

// ApplyType applies transformers to a type.
func ApplyType(t *mdl.Type, ts ...Transformer) *mdl.Type {
	return Compose(ts...).ApplyType(t)
}

// ApplyNode applies transformers to a node.
func ApplyNode(n mdl.Node, ts ...Transformer) mdl.Node {
	return Compose(ts...).ApplyNode(n)
}

package transform

import (
	mdl "github.com/viant/x/syntetic/model"
)

// NodeRewriter returns a Transformer that applies fn to every node, recursing
// into child nodes where applicable.
func NodeRewriter(fn func(mdl.Node) mdl.Node) Transformer {
	return nodeRewriter{fn: fn}
}

type nodeRewriter struct{ fn func(mdl.Node) mdl.Node }

func (n nodeRewriter) ApplyPackage(p *mdl.Package) *mdl.Package { return p }
func (n nodeRewriter) ApplyType(t *mdl.Type) *mdl.Type          { return t }
func (n nodeRewriter) ApplyNode(nd mdl.Node) mdl.Node {
	if nd == nil || n.fn == nil {
		return nd
	}
	// Recurse into composite nodes first, then apply rewrite to the node itself.
	switch v := nd.(type) {
	case *mdl.Pointer:
		v.Elem = n.ApplyNode(v.Elem)
	case *mdl.Slice:
		v.Elem = n.ApplyNode(v.Elem)
	case *mdl.Array:
		v.Elem = n.ApplyNode(v.Elem)
	case *mdl.Map:
		v.Key = n.ApplyNode(v.Key)
		v.Elem = n.ApplyNode(v.Elem)
	case *mdl.Chan:
		v.Elem = n.ApplyNode(v.Elem)
	case *mdl.Struct:
		if len(v.Fields) > 0 {
			out := make([]mdl.Field, len(v.Fields))
			for i, f := range v.Fields {
				f.Type = n.ApplyNode(f.Type)
				out[i] = f
			}
			v.Fields = out
		}
	case *mdl.Interface:
		// No child nodes; methods carry Func with child nodes but keep as-is here.
	case *mdl.Func:
		// Params/Results hold nodes; handled by a field rewriter if needed.
	}
	return n.fn(nd)
}

// FieldRewriter returns a Transformer that applies fn to all fields within
// struct nodes, allowing field modification or removal (return ok=false to drop).
func FieldRewriter(fn func(mdl.Field) (mdl.Field, bool)) Transformer {
	return fieldRewriter{fn: fn}
}

type fieldRewriter struct {
	fn func(mdl.Field) (mdl.Field, bool)
}

func (f fieldRewriter) ApplyPackage(p *mdl.Package) *mdl.Package { return p }
func (f fieldRewriter) ApplyType(t *mdl.Type) *mdl.Type          { return t }
func (f fieldRewriter) ApplyNode(n mdl.Node) mdl.Node {
	if n == nil || f.fn == nil {
		return n
	}
	st, ok := n.(*mdl.Struct)
	if !ok || st == nil {
		return n
	}
	out := &mdl.Struct{Fields: make([]mdl.Field, 0, len(st.Fields))}
	for _, fld := range st.Fields {
		nf, ok := f.fn(fld)
		if !ok {
			continue
		}
		out.Fields = append(out.Fields, nf)
	}
	return out
}

// TypeRewriter returns a Transformer that applies fn to types.
func TypeRewriter(fn func(*mdl.Type) *mdl.Type) Transformer { return typeRewriter{fn: fn} }

type typeRewriter struct{ fn func(*mdl.Type) *mdl.Type }

func (t typeRewriter) ApplyPackage(p *mdl.Package) *mdl.Package { return p }
func (t typeRewriter) ApplyType(tp *mdl.Type) *mdl.Type {
	if t.fn == nil || tp == nil {
		return tp
	}
	return t.fn(tp)
}
func (t typeRewriter) ApplyNode(n mdl.Node) mdl.Node { return n }

// DeclNameProvider exposes a declaration-name override for model.Type values.
// Builders can use this to override the printed name without mutating the type.
type DeclNameProvider interface {
	DeclNameFor(t *mdl.Type) (string, bool)
}

// DeclNameOverride returns a Transformer that provides a declaration-name
// override for types using the supplied function.
func DeclNameOverride(fn func(*mdl.Type) (string, bool)) Transformer {
	return declNameOverride{fn: fn}
}

type declNameOverride struct {
	fn func(*mdl.Type) (string, bool)
}

func (d declNameOverride) ApplyPackage(p *mdl.Package) *mdl.Package { return p }
func (d declNameOverride) ApplyType(t *mdl.Type) *mdl.Type          { return t }
func (d declNameOverride) ApplyNode(n mdl.Node) mdl.Node            { return n }
func (d declNameOverride) DeclNameFor(t *mdl.Type) (string, bool) {
	if d.fn == nil {
		return "", false
	}
	return d.fn(t)
}

// If a composed transformer (chain) contains a DeclNameProvider, expose the
// first override. This allows Compose(...) to be queried for overrides.
func (c chain) DeclNameFor(t *mdl.Type) (string, bool) {
	for _, tr := range c.list {
		if p, ok := tr.(DeclNameProvider); ok {
			if name, ok := p.DeclNameFor(t); ok {
				return name, true
			}
		}
	}
	return "", false
}

// LookupDeclName attempts to retrieve a decl-name override from a transformer,
// returning (name, true) when available.
func LookupDeclName(tr Transformer, t *mdl.Type) (string, bool) {
	if tr == nil || t == nil {
		return "", false
	}
	if p, ok := tr.(DeclNameProvider); ok {
		return p.DeclNameFor(t)
	}
	return "", false
}

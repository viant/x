package xreflect

import (
	"reflect"

	xtype2 "github.com/viant/x/loader/xreflect/xtype"
	"github.com/viant/x/syntetic/model"
)

// ToModelNode converts a reflect.Type into a syntetic/model.Node using the
// xtype intermediate representation. It propagates xtype.ErrUnnamedRecursion
// and returns (nil, nil) for reflect kinds that xtype does not model
// (for example, unsafe.Pointer).
func ToModelNode(rt reflect.Type) (model.Node, error) {
	if rt == nil {
		return nil, nil
	}
	xn, err := xtype2.FromReflect(rt)
	if err != nil {
		// ErrUnnamedRecursion is surfaced to callers so they can decide how to
		// handle unnamed recursive composites.
		return nil, err
	}
	if xn == nil {
		return nil, nil
	}
	return toModelNode(xn), nil
}

// toModelNode performs a shallow, shape-preserving conversion from an xtype
// Node into the syntetic/model Node. Unsupported kinds return nil.
func toModelNode(src xtype2.Node) model.Node {
	if src == nil {
		return nil
	}

	switch n := src.(type) {
	case *xtype2.Basic:
		return &model.Basic{Name: n.Name, PkgPath: n.PkgPath}

	case *xtype2.Named:
		// Treat builtin names (empty PkgPath) as basics for friendlier output.
		if n.PkgPath == "" {
			return &model.Basic{Name: n.Name}
		}
		// Skip unsafe.Pointer as unsupported in this bridge.
		if n.PkgPath == "unsafe" && n.Name == "Pointer" {
			return nil
		}
		// The current xtype reflection bridge models Underlying for named
		// types via the same Named node to handle cycles. To avoid infinite
		// recursion and mismatched expectations, we omit mapping Underlying
		// here. Downstream loaders that rely on AST should recover the
		// precise underlying shape when needed.
		return &model.Named{PkgPath: n.PkgPath, Name: n.Name}

	case *xtype2.Pointer:
		return &model.Pointer{Elem: toModelNode(n.Elem)}

	case *xtype2.Slice:
		return &model.Slice{Elem: toModelNode(n.Elem)}

	case *xtype2.Array:
		return &model.Array{Len: n.Len, Elem: toModelNode(n.Elem)}

	case *xtype2.Map:
		return &model.Map{Key: toModelNode(n.Key), Elem: toModelNode(n.Elem)}

	case *xtype2.Chan:
		return &model.Chan{Dir: n.Dir, Elem: toModelNode(n.Elem)}

	case *xtype2.Func:
		out := &model.Func{Variadic: n.Variadic}
		if len(n.Params) != 0 {
			out.Params = make([]model.Field, len(n.Params))
			for i, p := range n.Params {
				out.Params[i] = model.Field{
					Name:     p.Name,
					Type:     toModelNode(p.Type),
					Tag:      p.Tag,
					Embedded: p.Embedded,
				}
			}
		}
		if len(n.Results) != 0 {
			out.Results = make([]model.Field, len(n.Results))
			for i, r := range n.Results {
				out.Results[i] = model.Field{
					Name:     r.Name,
					Type:     toModelNode(r.Type),
					Tag:      r.Tag,
					Embedded: r.Embedded,
				}
			}
		}
		return out

	case *xtype2.Interface:
		if len(n.Methods) == 0 {
			return &model.Interface{}
		}
		methods := make([]model.Method, 0, len(n.Methods))
		for _, m := range n.Methods {
			// xtype.FromReflect only records exported methods; keep the
			// behaviour and trust the source here.
			fnNode, _ := toModelNode(&m.Type).(*model.Func)
			if fnNode == nil {
				continue
			}
			methods = append(methods, model.Method{Name: m.Name, Type: *fnNode})
		}
		return &model.Interface{Methods: methods}

	case *xtype2.Struct:
		if len(n.Fields) == 0 {
			return &model.Struct{}
		}
		fields := make([]model.Field, len(n.Fields))
		for i, f := range n.Fields {
			name := f.Name
			if f.Embedded {
				name = ""
			}
			fields[i] = model.Field{
				Name:     name,
				Type:     toModelNode(f.Type),
				Tag:      f.Tag,
				Embedded: f.Embedded,
			}
		}
		return &model.Struct{Fields: fields}
	}

	// Unsupported Node kinds (for example, xtype.Alias or future extensions)
	// are mapped to nil so that callers can treat them as absent types.
	return nil
}

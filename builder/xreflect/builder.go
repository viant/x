package xreflect

import (
	"fmt"
	"reflect"

	"github.com/viant/x/syntetic/model"
	trf "github.com/viant/x/syntetic/model/transform"
)

// Precomputed reflect.Type values for basic identifiers to avoid repeated
// calls to reflect.TypeOf during conversions.
var (
	rtBool       = reflect.TypeOf(false)
	rtString     = reflect.TypeOf("")
	rtInt        = reflect.TypeOf(int(0))
	rtInt8       = reflect.TypeOf(int8(0))
	rtInt16      = reflect.TypeOf(int16(0))
	rtInt32      = reflect.TypeOf(int32(0))
	rtInt64      = reflect.TypeOf(int64(0))
	rtUint       = reflect.TypeOf(uint(0))
	rtUint8      = reflect.TypeOf(uint8(0))
	rtUint16     = reflect.TypeOf(uint16(0))
	rtUint32     = reflect.TypeOf(uint32(0))
	rtUint64     = reflect.TypeOf(uint64(0))
	rtUintptr    = reflect.TypeOf(uintptr(0))
	rtFloat32    = reflect.TypeOf(float32(0))
	rtFloat64    = reflect.TypeOf(float64(0))
	rtComplex64  = reflect.TypeOf(complex64(0))
	rtComplex128 = reflect.TypeOf(complex128(0))
	rtError      = reflect.TypeOf((*error)(nil)).Elem()
)

var basicTypes = map[string]reflect.Type{
	"bool":       rtBool,
	"string":     rtString,
	"int":        rtInt,
	"int8":       rtInt8,
	"int16":      rtInt16,
	"int32":      rtInt32,
	"rune":       rtInt32,
	"int64":      rtInt64,
	"uint":       rtUint,
	"uint8":      rtUint8,
	"byte":       rtUint8,
	"uint16":     rtUint16,
	"uint32":     rtUint32,
	"uint64":     rtUint64,
	"uintptr":    rtUintptr,
	"float32":    rtFloat32,
	"float64":    rtFloat64,
	"complex64":  rtComplex64,
	"complex128": rtComplex128,
	"error":      rtError,
}

// Builder materializes reflect.Type from model.Node values using reflect's
// *Of constructors. It is stateful to hold caches and policies.
type Builder struct {
	resolver   Resolver
	namedCache map[string]reflect.Type
	unknown    func() reflect.Type
	allowIface bool
	tr         trf.Transformer
}

// New creates a new Builder with optional configuration.
func New(opts ...BuildOption) *Builder {
	b := &Builder{
		unknown: func() reflect.Type { return reflect.TypeOf((*interface{})(nil)).Elem() },
	}
	for _, o := range opts {
		if o != nil {
			o(b)
		}
	}
	if b.namedCache == nil {
		b.namedCache = map[string]reflect.Type{}
	}
	return b
}

// BuildNode converts a model.Node to a reflect.Type according to builder
// policies. It returns an error for shapes that cannot be expressed at
// runtime (e.g., anonymous recursive composites).
func (b *Builder) BuildNode(n model.Node) (reflect.Type, error) {
	if b.tr != nil {
		n = b.tr.ApplyNode(n)
	}
	return b.build(n, map[model.Node]bool{})
}

func (b *Builder) build(n model.Node, visiting map[model.Node]bool) (reflect.Type, error) {
	if n == nil {
		return nil, nil
	}
	switch v := n.(type) {
	case *model.Basic:
		return basicToReflect(v.Name), nil
	case *model.Named:
		// Resolve to existing compiled type.
		key := v.PkgPath + "." + v.Name
		if t, ok := b.namedCache[key]; ok {
			return t, nil
		}
		if b.resolver != nil {
			if t, ok := b.resolver.Resolve(v.PkgPath, v.Name); ok {
				b.namedCache[key] = t
				return t, nil
			}
		}
		// Fallback: if underlying provided, attempt anonymous build, else unknown
		if v.Underlying != nil {
			rt, err := b.build(v.Underlying, visiting)
			if err != nil {
				return nil, err
			}
			if rt != nil {
				return rt, nil
			}
		}
		return b.unknown(), nil
	case *model.Pointer:
		elem, err := b.build(v.Elem, visiting)
		if err != nil {
			return nil, err
		}
		if elem == nil {
			return nil, fmt.Errorf("xreflect: nil elem for pointer")
		}
		return reflect.PointerTo(elem), nil
	case *model.Slice:
		elem, err := b.build(v.Elem, visiting)
		if err != nil {
			return nil, err
		}
		if elem == nil {
			return nil, fmt.Errorf("xreflect: nil elem for slice")
		}
		return reflect.SliceOf(elem), nil
	case *model.Array:
		elem, err := b.build(v.Elem, visiting)
		if err != nil {
			return nil, err
		}
		if elem == nil {
			return nil, fmt.Errorf("xreflect: nil elem for array")
		}
		return reflect.ArrayOf(v.Len, elem), nil
	case *model.Map:
		key, err := b.build(v.Key, visiting)
		if err != nil {
			return nil, err
		}
		val, err := b.build(v.Elem, visiting)
		if err != nil {
			return nil, err
		}
		if key == nil || val == nil {
			return nil, fmt.Errorf("xreflect: nil key/value for map")
		}
		return reflect.MapOf(key, val), nil
	case *model.Chan:
		elem, err := b.build(v.Elem, visiting)
		if err != nil {
			return nil, err
		}
		if elem == nil {
			return nil, fmt.Errorf("xreflect: nil elem for chan")
		}
		return reflect.ChanOf(reflect.ChanDir(v.Dir), elem), nil
	case *model.Func:
		ins := make([]reflect.Type, len(v.Params))
		for i, p := range v.Params {
			rt, err := b.build(p.Type, visiting)
			if err != nil {
				return nil, err
			}
			if rt == nil {
				return nil, fmt.Errorf("xreflect: nil param type")
			}
			ins[i] = rt
		}
		outs := make([]reflect.Type, len(v.Results))
		for i, r := range v.Results {
			rt, err := b.build(r.Type, visiting)
			if err != nil {
				return nil, err
			}
			if rt == nil {
				return nil, fmt.Errorf("xreflect: nil result type")
			}
			outs[i] = rt
		}
		return reflect.FuncOf(ins, outs, v.Variadic), nil
	case *model.Interface:
		// For simplicity default to interface{} unless explicitly allowed.
		if !b.allowIface || len(v.Methods) == 0 {
			return reflect.TypeOf((*interface{})(nil)).Elem(), nil
		}
		// Attempt to construct an interface with the given method set.
		// Note: reflect.InterfaceOf has constraints; this may still fail
		// semantically for some shapes. We conservatively fall back to empty
		// interface by ignoring method names or errors here.
		methods := make([]reflect.Method, 0, len(v.Methods))
		for _, m := range v.Methods {
			mt, err := b.build(&m.Type, visiting)
			if err != nil {
				return nil, err
			}
			if mt == nil {
				return nil, fmt.Errorf("xreflect: nil method type")
			}
			methods = append(methods, reflect.Method{Name: m.Name, Type: mt})
		}
		// The reflect package currently does not expose a stable way to build
		// arbitrary named method interfaces across all versions. Use empty
		// interface for portability; advanced users can enable allowIface and
		// provide an Unknown fallback via Resolver as needed.
		return reflect.TypeOf((*interface{})(nil)).Elem(), nil
	case *model.Struct:
		if visiting[v] {
			return nil, fmt.Errorf("xreflect: anonymous recursive struct not supported")
		}
		visiting[v] = true
		defer delete(visiting, v)
		fields := make([]reflect.StructField, len(v.Fields))
		for i, f := range v.Fields {
			rt, err := b.build(f.Type, visiting)
			if err != nil {
				return nil, err
			}
			if rt == nil {
				return nil, fmt.Errorf("xreflect: nil field type for %q", f.Name)
			}
			sf := reflect.StructField{
				Name:      exportName(f.Name, f.Embedded),
				Type:      rt,
				Tag:       reflect.StructTag(f.Tag),
				Anonymous: f.Embedded,
			}
			fields[i] = sf
		}
		return reflect.StructOf(fields), nil
	case *model.Union:
		// No runtime representation
		return b.unknown(), nil
	}
	return b.unknown(), nil
}

func exportName(name string, embedded bool) string {
	if embedded || name == "" {
		return "_"
	} // placeholder; reflect.StructOf requires exported/valid names
	// Ensure first rune is upper-case to avoid package-private reject; if it
	// is lower-case, prefix with '_' to form a valid (though unexported) name
	r := rune(name[0])
	if r >= 'A' && r <= 'Z' {
		return name
	}
	return "_" + name
}

func basicToReflect(name string) reflect.Type { return basicTypes[name] }

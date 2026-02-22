package xreflect

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

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
	// strictNamed forces unresolved named nodes to return an error instead
	// of falling back to unknown().
	strictNamed bool
	// onUnresolvedNamed is an optional hook for diagnostics/telemetry.
	onUnresolvedNamed func(pkgPath, name string, candidates []string)
	// allowAnonRecursion controls handling of anonymous recursive structs.
	// When false, such shapes return an error (default behavior).
	// When true, recursive edge materializes via unknown() fallback.
	allowAnonRecursion bool
	// onAnonymousRecursion receives synthetic recursion identifiers when an
	// anonymous recursive edge is encountered in tolerant mode.
	onAnonymousRecursion func(name string)
	// synthetic recursion names for current BuildNode traversal.
	synthetic map[model.Node]string
	nextID    int
	// ifaceResolver resolves non-empty interface method sets to compiled interfaces.
	ifaceResolver InterfaceResolver
	// strictInterface forces unresolved non-empty interfaces to return an error
	// instead of falling back to unknown().
	strictInterface bool
	// onUnresolvedInterface is an optional diagnostics callback invoked when
	// strict interface resolution cannot resolve a non-empty interface.
	onUnresolvedInterface func(methods []model.Method)
	// strictUnion forces union/type-set nodes to return an error instead of
	// falling back to unknown().
	strictUnion bool
	// onUnresolvedUnion reports union terms when strict mode rejects them.
	onUnresolvedUnion func(terms []model.Term)
	// onAlias reports alias declaration metadata while runtime materialization
	// uses alias target type.
	onAlias func(alias *model.Alias, resolved reflect.Type)
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
	b.synthetic = map[model.Node]string{}
	b.nextID = 1
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
		if b.strictNamed {
			candidates := b.namedCandidates(v.Name)
			if b.onUnresolvedNamed != nil {
				b.onUnresolvedNamed(v.PkgPath, v.Name, candidates)
			}
			return nil, unresolvedNamedError(v.PkgPath, v.Name, candidates)
		}
		return b.unknown(), nil
	case *model.Alias:
		// Alias has no distinct runtime identity. Materialize target runtime type
		// and report alias metadata to caller.
		if v.Target == nil {
			return nil, fmt.Errorf("xreflect: alias %s.%s has nil target", v.PkgPath, v.Name)
		}
		rt, err := b.build(v.Target, visiting)
		if err != nil {
			return nil, err
		}
		if b.onAlias != nil {
			b.onAlias(v, rt)
		}
		return rt, nil
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
		// Empty interface always materializes to interface{}.
		if len(v.Methods) == 0 {
			return reflect.TypeOf((*interface{})(nil)).Elem(), nil
		}
		// Prefer explicit interface resolver for method-set fidelity.
		if b.ifaceResolver != nil {
			if t, ok := b.ifaceResolver.ResolveInterface(v.Methods); ok {
				return t, nil
			}
		}
		if b.strictInterface {
			if b.onUnresolvedInterface != nil {
				b.onUnresolvedInterface(v.Methods)
			}
			return nil, unresolvedInterfaceError(v.Methods)
		}
		// Legacy tolerant behavior: non-empty unresolved interfaces fallback.
		if !b.allowIface {
			return reflect.TypeOf((*interface{})(nil)).Elem(), nil
		}
		// reflect package does not offer a reliable cross-version constructor
		// for arbitrary non-empty interface method sets. Keep tolerant fallback.
		return reflect.TypeOf((*interface{})(nil)).Elem(), nil
	case *model.Struct:
		if visiting[v] {
			if b.allowAnonRecursion {
				name := b.syntheticName(v)
				if b.onAnonymousRecursion != nil {
					b.onAnonymousRecursion(name)
				}
				return b.unknown(), nil
			}
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
		// No native runtime representation for type-sets/unions.
		if b.strictUnion {
			if b.onUnresolvedUnion != nil {
				b.onUnresolvedUnion(v.Terms)
			}
			return nil, unresolvedUnionError(v.Terms)
		}
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

func (b *Builder) namedCandidates(name string) []string {
	if len(b.namedCache) == 0 {
		return nil
	}
	suffix := "." + name
	var candidates []string
	for k := range b.namedCache {
		if strings.HasSuffix(k, suffix) {
			candidates = append(candidates, k)
		}
	}
	sort.Strings(candidates)
	return candidates
}

func unresolvedNamedError(pkgPath, name string, candidates []string) error {
	if len(candidates) == 0 {
		return fmt.Errorf("xreflect: unresolved named type %s.%s", pkgPath, name)
	}
	return fmt.Errorf("xreflect: unresolved named type %s.%s; candidates: %s", pkgPath, name, strings.Join(candidates, ", "))
}

func unresolvedInterfaceError(methods []model.Method) error {
	if len(methods) == 0 {
		return fmt.Errorf("xreflect: unresolved interface")
	}
	names := make([]string, 0, len(methods))
	for _, m := range methods {
		names = append(names, m.Name)
	}
	sort.Strings(names)
	return fmt.Errorf("xreflect: unresolved non-empty interface; methods: %s", strings.Join(names, ", "))
}

func unresolvedUnionError(terms []model.Term) error {
	if len(terms) == 0 {
		return fmt.Errorf("xreflect: unresolved union/type-set constraint")
	}
	parts := NormalizeUnionTerms(terms)
	return fmt.Errorf("xreflect: unresolved union/type-set constraint: %s", strings.Join(parts, " | "))
}

// NormalizeUnionTerms stringifies union terms using stable formatting.
// Example output items: "~int", "string", "*example.com/p.Type".
func NormalizeUnionTerms(terms []model.Term) []string {
	if len(terms) == 0 {
		return nil
	}
	parts := make([]string, 0, len(terms))
	for _, t := range terms {
		item := "<?>"
		if t.Type != nil {
			item = nodeLabel(t.Type)
		}
		if t.Approx {
			item = "~" + item
		}
		parts = append(parts, item)
	}
	return parts
}

// JoinUnionTerms returns union terms joined with " | " for diagnostics.
func JoinUnionTerms(terms []model.Term) string {
	return strings.Join(NormalizeUnionTerms(terms), " | ")
}

func nodeLabel(n model.Node) string {
	switch v := n.(type) {
	case *model.Basic:
		if v.PkgPath != "" {
			return v.PkgPath + "." + v.Name
		}
		return v.Name
	case *model.Named:
		return v.PkgPath + "." + v.Name
	case *model.Pointer:
		return "*" + nodeLabel(v.Elem)
	case *model.Slice:
		return "[]" + nodeLabel(v.Elem)
	case *model.Array:
		return fmt.Sprintf("[%d]%s", v.Len, nodeLabel(v.Elem))
	case *model.Map:
		return "map[" + nodeLabel(v.Key) + "]" + nodeLabel(v.Elem)
	case *model.Chan:
		return "chan " + nodeLabel(v.Elem)
	case *model.Alias:
		return v.PkgPath + "." + v.Name
	default:
		return "<node>"
	}
}

func (b *Builder) syntheticName(n model.Node) string {
	if name, ok := b.synthetic[n]; ok {
		return name
	}
	name := fmt.Sprintf("AnonymousRecursive%d", b.nextID)
	b.nextID++
	b.synthetic[n] = name
	return name
}

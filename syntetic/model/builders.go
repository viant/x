package model

// Convenience builders for constructing Node graphs fluently.

// BasicType creates a basic/builtin type node.
func BasicType(name string) *Basic { return &Basic{Name: name} }

// NamedType creates a named type node with optional underlying node.
func NamedType(pkgPath, name string, underlying Node) *Named {
	return &Named{PkgPath: pkgPath, Name: name, Underlying: underlying}
}

// AliasType creates a type alias node.
func AliasType(pkgPath, name string, target Node) *Alias {
	return &Alias{PkgPath: pkgPath, Name: name, Target: target}
}

// PointerTo creates a pointer node.
func PointerTo(elem Node) *Pointer { return &Pointer{Elem: elem} }

// SliceOf creates a slice node.
func SliceOf(elem Node) *Slice { return &Slice{Elem: elem} }

// ArrayOf creates an array node.
func ArrayOf(length int, elem Node) *Array { return &Array{Len: length, Elem: elem} }

// MapOf creates a map node.
func MapOf(key, elem Node) *Map { return &Map{Key: key, Elem: elem} }

// ChanOf creates a channel node. Dir is compatible with ast.ChanDir.
func ChanOf(dir int, elem Node) *Chan { return &Chan{Dir: dir, Elem: elem} }

// NewStruct returns a builder for a struct node.
func NewStruct() *StructBuilder { return &StructBuilder{S: &Struct{}} }

// NewInterface returns a builder for an interface node.
func NewInterface() *InterfaceBuilder { return &InterfaceBuilder{I: &Interface{}} }

// NewFunc returns a builder for a function signature node.
func NewFunc() *FuncBuilder { return &FuncBuilder{F: &Func{}} }

// StructBuilder builds Struct nodes fluently.
type StructBuilder struct{ S *Struct }

// AddField appends a field to the struct. Duplicate named non-embedded
// fields are ignored (see Struct.AddField).
func (b *StructBuilder) AddField(name string, typ Node, tag string, embedded bool) *StructBuilder {
	if b == nil || b.S == nil {
		return b
	}
	b.S.AddField(Field{Name: name, Type: typ, Tag: tag, Embedded: embedded})
	return b
}

// Node returns the underlying Struct.
func (b *StructBuilder) Node() *Struct { return b.S }

// InterfaceBuilder builds Interface nodes fluently.
type InterfaceBuilder struct{ I *Interface }

// AddMethod appends a method if not present.
func (b *InterfaceBuilder) AddMethod(name string, fn Func) *InterfaceBuilder {
	if b == nil || b.I == nil {
		return b
	}
	b.I.AddMethod(Method{Name: name, Type: fn})
	return b
}

// AddMethodSig is a convenience to append a method using only parameter/result types.
func (b *InterfaceBuilder) AddMethodSig(name string, params []Node, results []Node, variadic bool) *InterfaceBuilder {
	fb := NewFunc()
	for _, p := range params {
		fb.AddParam(p)
	}
	fb.SetVariadic(variadic)
	for _, r := range results {
		fb.AddResult(r)
	}
	return b.AddMethod(name, *fb.Build())
}

// Node returns the underlying Interface.
func (b *InterfaceBuilder) Node() *Interface { return b.I }

// FuncBuilder builds Func nodes fluently.
type FuncBuilder struct{ F *Func }

// AddParam appends a parameter of type typ.
func (b *FuncBuilder) AddParam(typ Node) *FuncBuilder {
	if b == nil || b.F == nil {
		return b
	}
	b.F.Params = append(b.F.Params, Field{Type: typ})
	return b
}

// AddResult appends a result of type typ.
func (b *FuncBuilder) AddResult(typ Node) *FuncBuilder {
	if b == nil || b.F == nil {
		return b
	}
	b.F.Results = append(b.F.Results, Field{Type: typ})
	return b
}

// SetVariadic marks the function as variadic.
func (b *FuncBuilder) SetVariadic(v bool) *FuncBuilder {
	if b == nil || b.F == nil {
		return b
	}
	b.F.Variadic = v
	return b
}

// Build returns the underlying Func node.
func (b *FuncBuilder) Build() *Func { return b.F }

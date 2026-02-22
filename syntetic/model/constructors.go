package model

import "errors"

// Field options and constructor
type FieldOption func(*Field) error

func WithFieldName(name string) FieldOption {
	return func(f *Field) error { f.Name = name; return nil }
}
func WithFieldType(t Node) FieldOption {
	return func(f *Field) error { f.Type = t; return nil }
}
func WithFieldTag(tag string) FieldOption {
	return func(f *Field) error { f.Tag = tag; return nil }
}
func WithEmbedded(embedded bool) FieldOption {
	return func(f *Field) error { f.Embedded = embedded; return nil }
}

func NewField(opts ...FieldOption) (Field, error) {
	var f Field
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&f); err != nil {
			return Field{}, err
		}
	}
	if f.Type == nil {
		return Field{}, errors.New("model: field.Type is required")
	}
	if !f.Embedded && f.Name == "" {
		return Field{}, errors.New("model: field.Name is required for non-embedded field")
	}
	return f, nil
}

// Method options and constructor
type MethodOption func(*Method) error

func WithMethodName(name string) MethodOption {
	return func(m *Method) error { m.Name = name; return nil }
}
func WithMethodFunc(fn Func) MethodOption { return func(m *Method) error { m.Type = fn; return nil } }

func NewMethod(opts ...MethodOption) (Method, error) {
	var m Method
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&m); err != nil {
			return Method{}, err
		}
	}
	if m.Name == "" {
		return Method{}, errors.New("model: method.Name is required")
	}
	return m, nil
}

// Func options and constructor
type FuncOption func(*Func) error

func WithParam(t Node) FuncOption {
	return func(f *Func) error { f.Params = append(f.Params, Field{Type: t}); return nil }
}
func WithResult(t Node) FuncOption {
	return func(f *Func) error { f.Results = append(f.Results, Field{Type: t}); return nil }
}
func WithVariadic(v bool) FuncOption { return func(f *Func) error { f.Variadic = v; return nil } }

// MakeFunc constructs a function signature using functional options.
func MakeFunc(opts ...FuncOption) (*Func, error) {
	fn := &Func{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(fn); err != nil {
			return nil, err
		}
	}
	return fn, nil
}

// Node constructors via options for common shapes

// Basic
type BasicOption func(*Basic) error

func WithBasicName(n string) BasicOption { return func(b *Basic) error { b.Name = n; return nil } }
func WithBasicPkg(p string) BasicOption  { return func(b *Basic) error { b.PkgPath = p; return nil } }
func NewBasic(opts ...BasicOption) (*Basic, error) {
	b := &Basic{}
	for _, opt := range opts {
		if opt != nil {
			if err := opt(b); err != nil {
				return nil, err
			}
		}
	}
	if b.Name == "" {
		return nil, errors.New("model: basic.Name is required")
	}
	return b, nil
}

// Named
type NamedOption func(*Named) error

func WithNamedPkg(p string) NamedOption   { return func(n *Named) error { n.PkgPath = p; return nil } }
func WithNamedName(nm string) NamedOption { return func(n *Named) error { n.Name = nm; return nil } }
func WithNamedUnderlying(u Node) NamedOption {
	return func(n *Named) error { n.Underlying = u; return nil }
}
func NewNamed(opts ...NamedOption) (*Named, error) {
	n := &Named{}
	for _, opt := range opts {
		if opt != nil {
			if err := opt(n); err != nil {
				return nil, err
			}
		}
	}
	if n.Name == "" {
		return nil, errors.New("model: named.Name is required")
	}
	return n, nil
}

// Alias
type AliasOption func(*Alias) error

func WithAliasPkg(p string) AliasOption   { return func(a *Alias) error { a.PkgPath = p; return nil } }
func WithAliasName(nm string) AliasOption { return func(a *Alias) error { a.Name = nm; return nil } }
func WithAliasTarget(t Node) AliasOption  { return func(a *Alias) error { a.Target = t; return nil } }
func NewAlias(opts ...AliasOption) (*Alias, error) {
	a := &Alias{}
	for _, opt := range opts {
		if opt != nil {
			if err := opt(a); err != nil {
				return nil, err
			}
		}
	}
	if a.Name == "" || a.Target == nil {
		return nil, errors.New("model: alias requires Name and Target")
	}
	return a, nil
}

// Pointer
type PointerOption func(*Pointer) error

func WithElem(t Node) PointerOption { return func(p *Pointer) error { p.Elem = t; return nil } }
func NewPointer(opts ...PointerOption) (*Pointer, error) {
	p := &Pointer{}
	for _, opt := range opts {
		if opt != nil {
			if err := opt(p); err != nil {
				return nil, err
			}
		}
	}
	if p.Elem == nil {
		return nil, errors.New("model: pointer.Elem is required")
	}
	return p, nil
}

// Slice
type SliceOption func(*Slice) error

func WithSliceElem(t Node) SliceOption { return func(s *Slice) error { s.Elem = t; return nil } }
func NewSlice(opts ...SliceOption) (*Slice, error) {
	s := &Slice{}
	for _, opt := range opts {
		if opt != nil {
			if err := opt(s); err != nil {
				return nil, err
			}
		}
	}
	if s.Elem == nil {
		return nil, errors.New("model: slice.Elem is required")
	}
	return s, nil
}

// Array
type ArrayOption func(*Array) error

func WithArrayLen(n int) ArrayOption   { return func(a *Array) error { a.Len = n; return nil } }
func WithArrayElem(t Node) ArrayOption { return func(a *Array) error { a.Elem = t; return nil } }
func NewArray(opts ...ArrayOption) (*Array, error) {
	a := &Array{}
	for _, opt := range opts {
		if opt != nil {
			if err := opt(a); err != nil {
				return nil, err
			}
		}
	}
	if a.Len < 0 || a.Elem == nil {
		return nil, errors.New("model: array requires non-negative Len and Elem")
	}
	return a, nil
}

// Map
type MapOption func(*Map) error

func WithMapKey(k Node) MapOption  { return func(m *Map) error { m.Key = k; return nil } }
func WithMapElem(v Node) MapOption { return func(m *Map) error { m.Elem = v; return nil } }
func NewMap(opts ...MapOption) (*Map, error) {
	m := &Map{}
	for _, opt := range opts {
		if opt != nil {
			if err := opt(m); err != nil {
				return nil, err
			}
		}
	}
	if m.Key == nil || m.Elem == nil {
		return nil, errors.New("model: map requires Key and Elem")
	}
	return m, nil
}

// Chan
type ChanOption func(*Chan) error

func WithChanDir(dir int) ChanOption { return func(c *Chan) error { c.Dir = dir; return nil } }
func WithChanElem(t Node) ChanOption { return func(c *Chan) error { c.Elem = t; return nil } }
func NewChan(opts ...ChanOption) (*Chan, error) {
	c := &Chan{}
	for _, opt := range opts {
		if opt != nil {
			if err := opt(c); err != nil {
				return nil, err
			}
		}
	}
	if c.Elem == nil {
		return nil, errors.New("model: chan.Elem is required")
	}
	return c, nil
}

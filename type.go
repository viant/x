package x

import "reflect"

type (
	//Type represents a type
	Type struct {
		Type    reflect.Type
		Package string
		Name    string
		Scn     int
		Force   bool
		key     string
	}
)

// IsNamed returns true if type is named
func (t *Type) IsNamed() bool {
	typ := t.Type
	typ = SimpleType(typ)
	return typ.Name() != ""
}

// SimpleType returns a simple type, it unwraps pointer, slice, array, and chan
func SimpleType(typ reflect.Type) reflect.Type {
	switch typ.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Array, reflect.Chan:
		return SimpleType(typ.Elem())
	default:
		return typ
	}
}

// NewType creates a type
func NewType(t reflect.Type, options ...Option) *Type {
	typ := &Type{
		Type: t,
	}
	for _, opt := range options {
		opt(typ)
	}
	typ.init()
	return typ
}

func (t *Type) init() {
	if t.Package == "" {
		t.Package = t.Type.PkgPath()
	}
	if t.Name == "" {
		t.Name = t.Type.Name()
	}
	if t.Name == "" {
		t.Name = t.Type.String()
	}
}

func (t *Type) Key() string {
	if t.key != "" {
		return t.key
	}
	t.key = key(t)
	return t.key
}

func key(t *Type) string {
	pkg := t.Package
	name := t.Name
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

package xtype

type Kind int

const (
	KindUnknown Kind = iota
	KindBasic
	KindNamed
	KindAlias
	KindPointer
	KindSlice
	KindArray
	KindMap
	KindChan
	KindFunc
	KindInterface
	KindStruct
)

func (k Kind) String() string {
	switch k {
	case KindBasic:
		return "Basic"
	case KindNamed:
		return "Named"
	case KindAlias:
		return "Alias"
	case KindPointer:
		return "Pointer"
	case KindSlice:
		return "Slice"
	case KindArray:
		return "Array"
	case KindMap:
		return "Map"
	case KindChan:
		return "Chan"
	case KindFunc:
		return "Func"
	case KindInterface:
		return "Interface"
	case KindStruct:
		return "Struct"
	default:
		return "Unknown"
	}
}

type Node interface{ Kind() Kind }

type Basic struct{ Name, PkgPath string }

func (b *Basic) Kind() Kind { return KindBasic }

type Named struct {
	PkgPath, Name string
	Underlying    Node
}

func (n *Named) Kind() Kind { return KindNamed }

type Alias struct {
	PkgPath, Name string
	Target        Node
}

func (a *Alias) Kind() Kind { return KindAlias }

type Pointer struct{ Elem Node }

func (p *Pointer) Kind() Kind { return KindPointer }

type Slice struct{ Elem Node }

func (s *Slice) Kind() Kind { return KindSlice }

type Array struct {
	Len  int
	Elem Node
}

func (a *Array) Kind() Kind { return KindArray }

type Map struct{ Key, Elem Node }

func (m *Map) Kind() Kind { return KindMap }

type Chan struct {
	Dir  int
	Elem Node
}

func (c *Chan) Kind() Kind { return KindChan }

type Func struct {
	Params, Results []Field
	Variadic        bool
}

func (f *Func) Kind() Kind { return KindFunc }

type Interface struct{ Methods []Method }

func (i *Interface) Kind() Kind { return KindInterface }

type Struct struct{ Fields []Field }

func (s *Struct) Kind() Kind { return KindStruct }

type Field struct {
	Name     string
	Type     Node
	Tag      string
	Embedded bool
}
type Method struct {
	Name string
	Type Func
}

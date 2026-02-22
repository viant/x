package model

// Kind represents the specific shape of a Node.
//
// Stable numeric ordering is required for deterministic behaviour.
// Do not reorder existing values.
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
	KindUnion
)

// Node is a single type node in the synthetic model graph.
//
// All concrete nodes must be immutable once constructed.
type Node interface {
	Kind() Kind
}

// Basic represents built-in or package-local basic types.
// Example: int → &Basic{Name:"int"}
type Basic struct {
	Name    string
	PkgPath string // empty for builtins
}

// Kind returns the Kind of the Basic node.
func (n *Basic) Kind() Kind { return KindBasic }

// Named represents a defined type with a name (optionally generic).
// Example: type Box[T any] ... → &Named{PkgPath:"example.com/p", Name:"Box", TypeParams:[]TypeParam{{Name:"T", Constraint:&Basic{Name:"any"}}}}
type Named struct {
	PkgPath    string
	Name       string
	Underlying Node // may be nil during cycle resolution
	TypeParams []TypeParam
	// Methods declared on the named type (value receiver).
	Methods []Method
	// PtrMethods declared on the pointer to the named type (*T receiver).
	PtrMethods []Method
}

// Kind returns the Kind of the Named node.
func (n *Named) Kind() Kind { return KindNamed }

// HasMethod reports whether a method with the given name exists on
// the value receiver when ptr is false, or the pointer receiver when ptr is true.
func (n *Named) HasMethod(name string, ptr bool) bool {
	if n == nil || name == "" {
		return false
	}
	list := n.Methods
	if ptr {
		list = n.PtrMethods
	}
	for _, m := range list {
		if m.Name == name {
			return true
		}
	}
	return false
}

// AddMethod adds a method to the value receiver if it is not already present.
func (n *Named) AddMethod(m Method) {
	if n == nil || m.Name == "" || n.HasMethod(m.Name, false) {
		return
	}
	n.Methods = append(n.Methods, m)
}

// AddPtrMethod adds a method to the pointer receiver if not present.
func (n *Named) AddPtrMethod(m Method) {
	if n == nil || m.Name == "" || n.HasMethod(m.Name, true) {
		return
	}
	n.PtrMethods = append(n.PtrMethods, m)
}

// Alias represents an explicit type alias (optionally generic).
// Example: type IDs[T any] = []T → &Alias{Name:"IDs", TypeParams:[]TypeParam{{Name:"T", Constraint:&Basic{Name:"any"}}}, Target:&Slice{Elem:&Named{Name:"T"}}}
// Not produced by FromReflect but supported by ToAST.
type Alias struct {
	PkgPath    string
	Name       string
	TypeParams []TypeParam
	Target     Node
}

// Kind returns the Kind of the Alias node.
func (n *Alias) Kind() Kind { return KindAlias }

// Pointer represents a pointer type.
// Example: *T → &Pointer{Elem:&Named{Name:"T"}}
type Pointer struct {
	Elem Node
}

// Kind returns the Kind of the Pointer node.
func (n *Pointer) Kind() Kind { return KindPointer }

// Slice represents a slice type.
// Example: []int → &Slice{Elem:&Basic{Name:"int"}}
type Slice struct {
	Elem Node
}

// Kind returns the Kind of the Slice node.
func (n *Slice) Kind() Kind { return KindSlice }

// Array represents an array type.
// Example: [3]string → &Array{Len:3, Elem:&Basic{Name:"string"}}
type Array struct {
	Len  int
	Elem Node
}

// Kind returns the Kind of the Array node.
func (n *Array) Kind() Kind { return KindArray }

// Map represents a map type.
// Example: map[string]int → &Map{Key:&Basic{Name:"string"}, Elem:&Basic{Name:"int"}}
type Map struct {
	Key  Node
	Elem Node
}

// Kind returns the Kind of the Map node.
func (n *Map) Kind() Kind { return KindMap }

// Chan represents a channel type.
// Example: chan int → &Chan{Dir:int(token.SEND|token.RECV), Elem:&Basic{Name:"int"}}
// Dir is compatible with ast.ChanDir.
type Chan struct {
	Dir  int
	Elem Node
}

// Kind returns the Kind of the Chan node.
func (n *Chan) Kind() Kind { return KindChan }

// Func represents a function type.
// Example: func(x int) (string, error) → &Func{Params:[]Field{{Type:&Basic{Name:"int"}}}, Results:[]Field{{Type:&Basic{Name:"string"}},{Type:&Basic{Name:"error"}}}}
type Func struct {
	Params     []Field
	Results    []Field
	Variadic   bool
	TypeParams []TypeParam
}

// Kind returns the Kind of the Func node.
func (n *Func) Kind() Kind { return KindFunc }

// Interface represents an interface type with named methods.
// Example: interface{ Read([]byte) (int,error) }
type Interface struct {
	Methods []Method // exported only, flattened per reflect
}

// Kind returns the Kind of the Interface node.
func (n *Interface) Kind() Kind { return KindInterface }

// Term represents a single element in a union constraint; Approx indicates '~'.
// Example: ~int → Term{Type:&Basic{Name:"int"}, Approx:true}
type Term struct {
	Type   Node
	Approx bool
}

// Union represents a generic type union constraint (e.g. ~int | ~string).
// Example: ~int | string → &Union{Terms: []Term{{Type:&Basic{Name:"int"}, Approx:true}, {Type:&Basic{Name:"string"}}}}
type Union struct {
	Terms []Term
}

// Kind returns the Kind of the Union node.
func (n *Union) Kind() Kind { return KindUnion }

// HasMethod reports whether a method with the given name exists.
func (n *Interface) HasMethod(name string) bool {
	if n == nil || name == "" {
		return false
	}
	for _, m := range n.Methods {
		if m.Name == name {
			return true
		}
	}
	return false
}

// AddMethod appends a method if a method with the same name does not
// already exist.
func (n *Interface) AddMethod(m Method) {
	if n.HasMethod(m.Name) {
		return
	}
	n.Methods = append(n.Methods, m)
}

// Struct represents a struct type.
// Example: struct{ ID int; Name string }
type Struct struct {
	Fields []Field // declaration order preserved
}

// Kind returns the Kind of the Struct node.
func (n *Struct) Kind() Kind { return KindStruct }

// HasField reports whether a field with the given name exists.
// Only named (non-embedded) fields are considered.
func (n *Struct) HasField(name string) bool {
	if n == nil || name == "" {
		return false
	}
	for _, f := range n.Fields {
		if !f.Embedded && f.Name == name {
			return true
		}
	}
	return false
}

// AddField appends a field if a non-embedded field with the same name
// does not already exist. Embedded fields are not de-duplicated.
func (n *Struct) AddField(f Field) {
	if !f.Embedded && f.Name != "" {
		if n.HasField(f.Name) {
			return
		}
	}
	n.Fields = append(n.Fields, f)
}

// Field describes a struct field or function parameter/result.
//
// Field is a helper value; it is not a Node.
type Field struct {
	Name     string // empty for embedded fields
	Type     Node
	Tag      string // raw, without backticks
	Embedded bool
}

// Method describes an interface or named type method.
//
// Method is a helper value; it is not a Node.
type Method struct {
	Name string
	Type Func
}

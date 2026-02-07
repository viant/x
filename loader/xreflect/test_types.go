package xreflect

// This file defines local types used by reflect_to_model_test.go. The goal is
// to exercise the reflect → xtype → syntetic/model.Node conversion across a
// representative subset of Go kinds that are supported by xtype.FromReflect.

type BasicStruct struct {
	Int   int
	Str   string
	Named NamedInt
	Ptr   *int
	Slice []string
	Array [2]bool
	Map   map[string]int
	Recv  chan<- int
	Send  <-chan string
	Both  chan bool
}

type NamedInt int

type EmbeddedA struct {
	ID int `json:"id"`
}

type EmbeddedB struct {
	EmbeddedA
	Name string `yaml:"name"`
}

// Func types covering parameters, results, and variadic flag.
type FuncSimple func(int) string

type FuncVariadic func(prefix string, values ...int) (int, error)

// Interfaces with exported and unexported methods.
type InterfaceMixed interface {
	Exported(a int) string
	unexported()
}

// Named recursive type; FromReflect may treat deep unnamed recursion as
// unsupported. We still include a simple self-referential named struct
// to cover the basic case that is representable.
type NamedRecursive struct {
	Next *NamedRecursive
}

// Additional types for extended coverage
type K string
type V struct{ N int }

type E struct {
	ID int `json:"id"`
}

type A struct{ B *B }
type B struct{ A *A }

// Named interface for coverage
type R interface{ Error() string }

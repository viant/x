package xreflect

import (
	"reflect"
	"testing"
	"unsafe"

	smodel "github.com/viant/x/syntetic/model"
)

// TestToModelNode_ReflectInterop exercises ToModelNode over a selection of
// Go kinds that are supported by xtype.FromReflect. The tests are
// intentionally golden-ish: we construct the expected syntetic/model.Node
// graphs manually and compare them structurally via equalNodes.
//
// Notes on limitations:
//   - reflect exposes only exported methods on interface types; unexported
//     methods are not visible and therefore not included in the resulting
//     model.Interface.Methods slice.
//   - Some unusual unnamed recursive composites are rejected by
//     xtype.FromReflect with xtype.ErrUnnamedRecursion. Those shapes are
//     difficult to construct directly via reflect.Type and are covered at
//     the xtype layer instead of here.
func TestToModelNode_ReflectInterop(t *testing.T) {
	type iface = InterfaceMixed

	tests := []struct {
		name    string
		in      reflect.Type
		want    smodel.Node
		wantErr error
	}{
		// Basics and simple composites
		{
			name: "basic-int",
			in:   reflect.TypeOf(int(0)),
			want: &smodel.Basic{Name: "int"},
		},
		{
			name: "named-int",
			in:   reflect.TypeOf(NamedInt(0)),
			want: &smodel.Named{PkgPath: reflect.TypeOf(NamedInt(0)).PkgPath(), Name: "NamedInt"},
		},
		{
			name: "pointer",
			in:   reflect.TypeOf((*int)(nil)),
			want: &smodel.Pointer{Elem: &smodel.Basic{Name: "int"}},
		},
		{
			name: "slice",
			in:   reflect.TypeOf([]string{}),
			want: &smodel.Slice{Elem: &smodel.Basic{Name: "string"}},
		},
		{
			name: "array",
			in:   reflect.TypeOf([2]bool{}),
			want: &smodel.Array{Len: 2, Elem: &smodel.Basic{Name: "bool"}},
		},
		{
			name: "map",
			in:   reflect.TypeOf(map[string]int{}),
			want: &smodel.Map{Key: &smodel.Basic{Name: "string"}, Elem: &smodel.Basic{Name: "int"}},
		},
		{
			name: "chan-both",
			in:   reflect.TypeOf((chan bool)(nil)),
			want: &smodel.Chan{Dir: int(reflect.BothDir), Elem: &smodel.Basic{Name: "bool"}},
		},
		// Functions, structs, interfaces
		{
			name: "func-simple",
			in:   reflect.TypeOf((func(int) string)(nil)),
			want: &smodel.Func{
				Params:  []smodel.Field{{Type: &smodel.Basic{Name: "int"}}},
				Results: []smodel.Field{{Type: &smodel.Basic{Name: "string"}}},
			},
		},
		{
			name: "func-variadic",
			in:   reflect.TypeOf((func(prefix string, values ...int) (int, error))(nil)),
			want: &smodel.Func{
				Params: []smodel.Field{
					{Type: &smodel.Basic{Name: "string"}},
					{Type: &smodel.Slice{Elem: &smodel.Basic{Name: "int"}}},
				},
				Results: []smodel.Field{
					{Type: &smodel.Basic{Name: "int"}},
					{Type: &smodel.Basic{Name: "error", PkgPath: ""}},
				},
				Variadic: true,
			},
		},
		{
			name: "struct-with-tags-embedded",
			in: reflect.TypeOf(struct {
				EmbeddedA
				Name string `yaml:"name"`
			}{}),
			want: &smodel.Struct{
				Fields: []smodel.Field{
					{
						Name:     "",
						Embedded: true,
						Type:     &smodel.Named{PkgPath: reflect.TypeOf(EmbeddedA{}).PkgPath(), Name: "EmbeddedA"},
					},
					{Name: "Name", Tag: "yaml:\"name\"", Type: &smodel.Basic{Name: "string"}},
				},
			},
		},
		{
			name: "interface-exported-method-only",
			in: reflect.TypeOf((*interface {
				Exported(a int) string
				unexported()
			})(nil)).Elem(),
			want: &smodel.Interface{
				Methods: []smodel.Method{
					{
						Name: "Exported",
						Type: smodel.Func{
							Params:  []smodel.Field{{Type: &smodel.Basic{Name: "int"}}},
							Results: []smodel.Field{{Type: &smodel.Basic{Name: "string"}}},
						},
					},
				},
			},
		},
		// Recursion and unsupported
		{
			name: "named-recursive",
			in:   reflect.TypeOf(NamedRecursive{}),
			want: &smodel.Named{PkgPath: reflect.TypeOf(NamedRecursive{}).PkgPath(), Name: "NamedRecursive"},
		},
		{
			name: "unsafe-pointer-unsupported",
			in:   reflect.TypeOf(unsafe.Pointer(nil)),
			want: nil, // unsupported kind → (nil, nil)
		},

		// Additional nested and advanced shapes
		{
			name: "nested-map-slice-map-array",
			in:   reflect.TypeOf(map[string][]map[int][3]uint8{}),
			want: &smodel.Map{
				Key:  &smodel.Basic{Name: "string"},
				Elem: &smodel.Slice{Elem: &smodel.Map{Key: &smodel.Basic{Name: "int"}, Elem: &smodel.Array{Len: 3, Elem: &smodel.Basic{Name: "uint8"}}}},
			},
		},
		{
			name: "deep-pointers",
			in:   reflect.TypeOf((***int)(nil)),
			want: &smodel.Pointer{Elem: &smodel.Pointer{Elem: &smodel.Pointer{Elem: &smodel.Basic{Name: "int"}}}},
		},
		{
			name: "pointer-to-array-of-ptr",
			in:   reflect.TypeOf((*[2]*string)(nil)),
			want: &smodel.Pointer{Elem: &smodel.Array{Len: 2, Elem: &smodel.Pointer{Elem: &smodel.Basic{Name: "string"}}}},
		},
		{
			name: "slice-of-funcs",
			in:   reflect.TypeOf([]func(error) int(nil)),
			want: &smodel.Slice{Elem: &smodel.Func{Params: []smodel.Field{{Type: &smodel.Basic{Name: "error"}}}, Results: []smodel.Field{{Type: &smodel.Basic{Name: "int"}}}}},
		},
		{
			name: "func-no-params-results",
			in:   reflect.TypeOf((func())(nil)),
			want: &smodel.Func{Params: nil, Results: nil},
		},
		{
			name: "func-with-chans",
			in:   reflect.TypeOf((func(chan<- int) (<-chan string, error))(nil)),
			want: &smodel.Func{
				Params:  []smodel.Field{{Type: &smodel.Chan{Dir: int(reflect.SendDir), Elem: &smodel.Basic{Name: "int"}}}},
				Results: []smodel.Field{{Type: &smodel.Chan{Dir: int(reflect.RecvDir), Elem: &smodel.Basic{Name: "string"}}}, {Type: &smodel.Basic{Name: "error"}}},
			},
		},
		{
			name: "chan-send-only",
			in:   reflect.TypeOf((chan<- int)(nil)),
			want: &smodel.Chan{Dir: int(reflect.SendDir), Elem: &smodel.Basic{Name: "int"}},
		},
		{
			name: "chan-recv-only",
			in:   reflect.TypeOf((<-chan int)(nil)),
			want: &smodel.Chan{Dir: int(reflect.RecvDir), Elem: &smodel.Basic{Name: "int"}},
		},
		{
			name: "map-with-named-key-value",
			in:   reflect.TypeOf(map[K]V{}),
			want: &smodel.Map{Key: &smodel.Named{PkgPath: reflect.TypeOf(K("")).PkgPath(), Name: "K"}, Elem: &smodel.Named{PkgPath: reflect.TypeOf(V{}).PkgPath(), Name: "V"}},
		},
		{
			name: "struct-pointer-embedded-and-tags",
			in: reflect.TypeOf(struct {
				*E
				Name string `yaml:"n" xml:"name"`
			}{}),
			want: &smodel.Struct{Fields: []smodel.Field{
				{Embedded: true, Name: "", Type: &smodel.Pointer{Elem: &smodel.Named{PkgPath: reflect.TypeOf(E{}).PkgPath(), Name: "E"}}},
				{Name: "Name", Tag: "yaml:\"n\" xml:\"name\"", Type: &smodel.Basic{Name: "string"}},
			}},
		},
		{
			name: "struct-with-anon-interface-field",
			in: reflect.TypeOf(struct {
				I interface{ Read([]uint8) (int, error) }
			}{}),
			want: &smodel.Struct{Fields: []smodel.Field{{Name: "I", Type: &smodel.Interface{Methods: []smodel.Method{{
				Name: "Read",
				Type: smodel.Func{Params: []smodel.Field{{Type: &smodel.Slice{Elem: &smodel.Basic{Name: "uint8"}}}}, Results: []smodel.Field{{Type: &smodel.Basic{Name: "int"}}, {Type: &smodel.Basic{Name: "error"}}}},
			}}}}}},
		},
		{
			name: "empty-interface",
			in:   reflect.TypeOf((*interface{})(nil)).Elem(),
			want: &smodel.Interface{},
		},
		{
			name: "named-interface",
			in:   reflect.TypeOf((*R)(nil)).Elem(),
			want: &smodel.Named{PkgPath: reflect.TypeOf((*R)(nil)).Elem().PkgPath(), Name: "R"},
		},
		{
			name: "mutual-recursion-A",
			in:   reflect.TypeOf(A{}),
			want: &smodel.Named{PkgPath: reflect.TypeOf(A{}).PkgPath(), Name: "A"},
		},
		{
			name: "mutual-recursion-B",
			in:   reflect.TypeOf(B{}),
			want: &smodel.Named{PkgPath: reflect.TypeOf(B{}).PkgPath(), Name: "B"},
		},
		{
			name: "array-len-0",
			in:   reflect.TypeOf([0]int{}),
			want: &smodel.Array{Len: 0, Elem: &smodel.Basic{Name: "int"}},
		},
		{
			name: "array-len-1-struct-empty",
			in:   reflect.TypeOf([1]struct{}{}),
			want: &smodel.Array{Len: 1, Elem: &smodel.Struct{}},
		},
		{
			name: "map-pointer-key-value",
			in:   reflect.TypeOf(map[*int]*string{}),
			want: &smodel.Map{Key: &smodel.Pointer{Elem: &smodel.Basic{Name: "int"}}, Elem: &smodel.Pointer{Elem: &smodel.Basic{Name: "string"}}},
		},
		{
			name: "slice-of-recv-chan-of-bytes",
			in:   reflect.TypeOf([]<-chan []uint8(nil)),
			want: &smodel.Slice{Elem: &smodel.Chan{Dir: int(reflect.RecvDir), Elem: &smodel.Slice{Elem: &smodel.Basic{Name: "uint8"}}}},
		},
		{
			name: "unnamed-struct-top-level",
			in:   reflect.TypeOf(struct{ X int }{}),
			want: &smodel.Struct{Fields: []smodel.Field{{Name: "X", Type: &smodel.Basic{Name: "int"}}}},
		},
		{
			name: "uintptr-basic",
			in:   reflect.TypeOf(uintptr(0)),
			want: &smodel.Basic{Name: "uintptr"},
		},
		{
			name: "complex128-basic",
			in:   reflect.TypeOf(complex128(0)),
			want: &smodel.Basic{Name: "complex128"},
		},
		// Nested recursion shapes using self-references inside composites
		{
			name: "slice-of-self-named",
			in:   reflect.TypeOf([]*NamedRecursive(nil)),
			want: &smodel.Slice{Elem: &smodel.Pointer{Elem: &smodel.Named{PkgPath: reflect.TypeOf(NamedRecursive{}).PkgPath(), Name: "NamedRecursive"}}},
		},
		{
			name: "map-of-self-named",
			in:   reflect.TypeOf(map[string]*NamedRecursive{}),
			want: &smodel.Map{Key: &smodel.Basic{Name: "string"}, Elem: &smodel.Pointer{Elem: &smodel.Named{PkgPath: reflect.TypeOf(NamedRecursive{}).PkgPath(), Name: "NamedRecursive"}}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToModelNode(tt.in)
			if tt.wantErr != nil {
				if err == nil || err.Error() != tt.wantErr.Error() {
					t.Fatalf("unexpected error: got=%v, want=%v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ToModelNode(%v) error: %v", tt.in, err)
			}
			equal, msg := equalNodes(got, tt.want)
			if !equal {
				t.Fatalf("ToModelNode(%v) mismatch: %s", tt.in, msg)
			}
		})
	}
}

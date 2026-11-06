package shape

import (
	"encoding"
	"encoding/json"
	"fmt"
	"go/token"
	"reflect"
	"sync"
)

// FieldMetadata is the editable part of a field. Storage type, position,
// embedding and package authority are deliberately not editable.
type FieldMetadata struct {
	// SkipChildren is traversal control only; it is never stored as schema metadata.
	SkipChildren bool
	Name         string
	Tag          reflect.StructTag
}

// FieldRewriter receives immutable original metadata. A nil result means no
// change. parentPath excludes the current field name and uses [] for collection
// elements and {} for map values. Map keys and interface implementations are not
// traversed. The parent is always the original linked containing struct.
type FieldRewriter func(parentPath string, parent reflect.Type, field *FieldMetadata) (*FieldMetadata, error)

// RewritePlan holds immutable direct-field traversal metadata. Cache this plan
// beside its original linked type; concurrent Rewrite calls share no edit state.
// It does not cache caller policy decisions or rewritten variants.
type RewritePlan struct {
	root               *rewriteNode
	representationOnce sync.Once
	representation     *Representation
}

type rewriteNode struct {
	backEdge bool
	original reflect.Type
	path     string
	fields   []rewriteField
	elem     *rewriteNode
	cycle    bool
}
type rewriteField struct {
	original reflect.StructField
	metadata FieldMetadata
	child    *rewriteNode
}

// CompileRewrite compiles direct structural traversal, preserving anonymous
// holders rather than using VisibleFields/JSONFields, which promote fields.
func CompileRewrite(original reflect.Type) (*RewritePlan, error) {
	if original == nil {
		return nil, fmt.Errorf("original type is required")
	}
	active := map[reflect.Type]int{}
	var stack []*rewriteNode
	var compile func(reflect.Type, string) *rewriteNode
	compile = func(t reflect.Type, path string) *rewriteNode {
		n := &rewriteNode{original: t, path: path}
		if index, ok := active[t]; ok {
			n.backEdge = true
			for _, ancestor := range stack[index:] {
				ancestor.cycle = true
			}
			return n // Back-edge retains linked authority; never synthesize recursion.
		}
		active[t] = len(stack)
		stack = append(stack, n)
		defer func() { delete(active, t); stack = stack[:len(stack)-1] }()
		switch t.Kind() {
		case reflect.Pointer:
			n.elem = compile(t.Elem(), path)
		case reflect.Slice, reflect.Array:
			n.elem = compile(t.Elem(), path+"[]")
		case reflect.Map:
			n.elem = compile(t.Elem(), path+"{}")
		case reflect.Struct:
			n.fields = make([]rewriteField, t.NumField())
			for i := range n.fields {
				f := t.Field(i)
				childPath := f.Name
				if path != "" {
					childPath = path + "." + f.Name
				}
				n.fields[i] = rewriteField{original: f, metadata: FieldMetadata{Name: f.Name, Tag: f.Tag}}
				// Private storage is preserved. An unexported embedded holder may
				// promote public fields, so its descendants still participate.
				if f.IsExported() || f.Anonymous {
					n.fields[i].child = compile(f.Type, childPath)
				}
			}
		}
		return n
	}
	return &RewritePlan{root: compile(original, "")}, nil
}

func (p *RewritePlan) Original() reflect.Type {
	if p == nil || p.root == nil {
		return nil
	}
	return p.root.original
}

// VisitStructs visits original structural owners in parent-first order, including
// owners without fields and owners inside pointer/slice/array/map-value wrappers.
// Paths are identical to the parent paths supplied to FieldRewriter. Recursive
// back-edges are not revisited. The visitor never receives mutable plan state.
func (p *RewritePlan) VisitStructs(visit func(path string, original reflect.Type) error) error {
	if p == nil || p.root == nil {
		return fmt.Errorf("rewrite plan is required")
	}
	if visit == nil {
		return nil
	}
	return p.root.visitStructs(visit)
}
func (n *rewriteNode) visitStructs(visit func(string, reflect.Type) error) error {
	if n.backEdge {
		return nil
	}
	if n.original.Kind() == reflect.Struct {
		if err := visit(n.path, n.original); err != nil {
			return err
		}
	}
	if n.elem != nil {
		return n.elem.visitStructs(visit)
	}
	for i := range n.fields {
		if child := n.fields[i].child; child != nil {
			if err := child.visitStructs(visit); err != nil {
				return err
			}
		}
	}
	return nil
}

// Rewrite returns the exact linked type at every unchanged node, even when a
// sibling changes. Only changed ancestors are reconstructed. The result is an
// effective representation, not a replacement application value/receiver type.
// Changing recursive type cycles or custom encoded types requires an explicit
// codec projection and returns an error; no unsafe value casting is performed.
func (p *RewritePlan) Rewrite(rewrite FieldRewriter) (result reflect.Type, err error) {
	if p == nil || p.root == nil {
		return nil, fmt.Errorf("rewrite plan is required")
	}
	if rewrite == nil {
		return p.root.original, nil
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			err = fmt.Errorf("rewrite type %s: %v", p.root.original, recovered)
		}
	}()
	return p.root.rewrite(rewrite)
}

func (n *rewriteNode) rewrite(rewrite FieldRewriter) (reflect.Type, error) {
	t := n.original
	if n.elem != nil {
		elem, err := n.elem.rewrite(rewrite)
		if err != nil {
			return nil, err
		}
		if elem == t.Elem() {
			return t, nil
		}
		if err = n.allowChange(); err != nil {
			return nil, err
		}
		switch t.Kind() {
		case reflect.Pointer:
			return reflect.PointerTo(elem), nil
		case reflect.Slice:
			return reflect.SliceOf(elem), nil
		case reflect.Array:
			return reflect.ArrayOf(t.Len(), elem), nil
		case reflect.Map:
			return reflect.MapOf(t.Key(), elem), nil
		}
	}
	var fields []RuntimeField
	for i := range n.fields {
		field := &n.fields[i]
		name, tag := field.original.Name, field.original.Tag
		skipChildren := false
		if field.original.IsExported() {
			edit, err := rewrite(n.path, t, &field.metadata)
			if err != nil {
				return nil, fmt.Errorf("field %s.%s: %w", n.path, name, err)
			}
			if edit != nil {
				if edit.Name != name && (!token.IsIdentifier(edit.Name) || !token.IsExported(edit.Name) || field.original.Anonymous) {
					return nil, fmt.Errorf("invalid field rename %s.%s to %q", n.path, name, edit.Name)
				}
				name, tag = edit.Name, edit.Tag
				skipChildren = edit.SkipChildren
			}
		}
		child := field.original.Type
		if field.child != nil && !skipChildren {
			var err error
			child, err = field.child.rewrite(rewrite)
			if err != nil {
				return nil, err
			}
		}
		if name == field.original.Name && tag == field.original.Tag && child == field.original.Type {
			continue
		}
		if fields == nil {
			if err := n.allowChange(); err != nil {
				return nil, err
			}
			fields = make([]RuntimeField, len(n.fields))
			for j := range n.fields {
				f := n.fields[j].original
				fields[j] = RuntimeField{Name: f.Name, Type: f.Type, Tag: f.Tag, Anonymous: f.Anonymous, PkgPath: f.PkgPath}
			}
		}
		fields[i].Name, fields[i].Tag, fields[i].Type = name, tag, child
	}
	if fields == nil {
		return t, nil
	}
	result, err := (Runtime{}).Struct(fields)
	if err != nil {
		return nil, fmt.Errorf("type %s at %s: %w", t, n.path, err)
	}
	if result.Size() != t.Size() || result.Align() != t.Align() {
		return nil, fmt.Errorf("rewrite changed layout of %s", t)
	}
	for i := range n.fields {
		if result.Field(i).Offset != n.fields[i].original.Offset {
			return nil, fmt.Errorf("rewrite changed field offset in %s", t)
		}
	}
	return result, nil
}

var rewriteOpaqueInterfaces = []reflect.Type{
	reflect.TypeOf((*json.Marshaler)(nil)).Elem(), reflect.TypeOf((*json.Unmarshaler)(nil)).Elem(),
	reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem(), reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem(),
	reflect.TypeOf((*interface{ IsZero() bool })(nil)).Elem(),
}

func (n *rewriteNode) allowChange() error { return n.allowChangeWith(rewriteOpaqueInterfaces) }

// Metadata consumers keep the original IsZero receiver. Complete JSON/Text
// owners still control their own representation and cannot be retagged inside.
func (n *rewriteNode) allowMetadataChange() error {
	return n.allowChangeWith(rewriteOpaqueInterfaces[:len(rewriteOpaqueInterfaces)-1])
}
func (n *rewriteNode) allowChangeWith(opaque []reflect.Type) error {
	if n.cycle {
		return fmt.Errorf("rewriting recursive type %s at %s requires a codec projection", n.original, n.path)
	}
	for _, codec := range opaque {
		if n.original.Implements(codec) || reflect.PointerTo(n.original).Implements(codec) {
			return fmt.Errorf("rewriting custom encoded type %s at %s requires a codec projection", n.original, n.path)
		}
	}
	return nil
}

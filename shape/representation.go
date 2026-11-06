package shape

import (
	"fmt"
	"go/token"
	"reflect"
	"strconv"
	"strings"
)

// Representation carries immutable field metadata separately from linked storage.
// Original always remains the canonical type, including its method set. Fields
// retain their canonical types, indexes, offsets and embedding flags; FieldShape
// supplies each child's effective metadata. No runtime type is constructed.
type Representation struct {
	node     *rewriteNode
	original reflect.Type
	fields   []representationField
	element  *Representation
	changed  bool
}
type representationField struct {
	original reflect.StructField
	metadata FieldMetadata
	shape    *Representation
}

func (r *Representation) Original() reflect.Type               { return r.original }
func (r *Representation) Changed() bool                        { return r != nil && r.changed }
func (r *Representation) NumField() int                        { return len(r.fields) }
func (r *Representation) Elem() *Representation                { return r.element }
func (r *Representation) FieldShape(index int) *Representation { return r.fields[index].shape }
func (r *Representation) Field(index int) reflect.StructField {
	f := r.fields[index]
	result := f.original
	result.Name, result.Tag = f.metadata.Name, f.metadata.Tag
	result.Index = append([]int(nil), result.Index...)
	return result
}

// RewriteRepresentation uses the same canonical traversal and opaque-codec
// boundaries as Rewrite, without reflect.StructOf's embedded-method limitation.
// Nil/equal edits reuse the original representation at every unchanged node.
func (p *RewritePlan) RewriteRepresentation(rewrite FieldRewriter) (*Representation, error) {
	if p == nil || p.root == nil {
		return nil, fmt.Errorf("rewrite plan is required")
	}
	p.representationOnce.Do(func() { p.representation = linkedRepresentation(p.root) })
	if rewrite == nil {
		return p.representation, nil
	}
	return p.representation.rewrite(rewrite)
}
func linkedRepresentation(node *rewriteNode) *Representation {
	result := &Representation{node: node, original: node.original}
	if node.elem != nil {
		result.element = linkedRepresentation(node.elem)
	}
	if len(node.fields) > 0 {
		result.fields = make([]representationField, len(node.fields))
		for i, field := range node.fields {
			child := &Representation{original: field.original.Type}
			if field.child != nil {
				child = linkedRepresentation(field.child)
			}
			result.fields[i] = representationField{original: field.original, metadata: field.metadata, shape: child}
		}
	}
	return result
}
func (r *Representation) rewrite(rewrite FieldRewriter) (*Representation, error) {
	if r.node == nil || r.node.backEdge {
		return r, nil
	}
	result := r
	if r.element != nil {
		child, err := r.element.rewrite(rewrite)
		if err != nil {
			return nil, err
		}
		if child != r.element {
			if err = r.node.allowMetadataChange(); err != nil {
				return nil, err
			}
			copy := *r
			copy.element = child
			copy.changed = true
			result = &copy
		}
		return result, nil
	}
	for i, field := range r.fields {
		metadata := field.metadata
		if field.original.IsExported() {
			edit, err := rewrite(r.node.path, r.original, &metadata)
			if err != nil {
				return nil, fmt.Errorf("field %s.%s: %w", r.node.path, field.original.Name, err)
			}
			selected := field.metadata
			if edit != nil {
				selected = *edit
			}
			metadata = selected
			if metadata.Name != field.original.Name && (!token.IsIdentifier(metadata.Name) || !token.IsExported(metadata.Name) || field.original.Anonymous) {
				return nil, fmt.Errorf("invalid field rename %s.%s to %q", r.node.path, field.original.Name, metadata.Name)
			}
		}
		skipChildren := metadata.SkipChildren
		metadata.SkipChildren = false
		child := field.shape
		var err error
		if !skipChildren {
			child, err = field.shape.rewrite(rewrite)
			if err != nil {
				return nil, err
			}
		}
		if metadata == field.metadata && child == field.shape {
			continue
		}
		if result == r {
			if err = r.node.allowMetadataChange(); err != nil {
				return nil, err
			}
			copy := *r
			copy.fields = append([]representationField(nil), r.fields...)
			copy.changed = true
			result = &copy
		}
		result.fields[i].metadata = metadata
		result.fields[i].shape = child
	}
	if result != r {
		names := map[string]bool{}
		for _, field := range result.fields {
			if names[field.metadata.Name] {
				return nil, fmt.Errorf("duplicate field %s in %s", field.metadata.Name, r.original)
			}
			names[field.metadata.Name] = true
		}
	}
	return result, nil
}

// Reflected materializes a representation when Go reflection can express it.
// Consumers supporting overlays must use Original/Field/FieldShape when this
// returns a reflection restriction. No-change nodes return exact linked types.
func (r *Representation) Reflected() (reflect.Type, error) {
	if r == nil {
		return nil, fmt.Errorf("representation is required")
	}
	if !r.changed {
		return r.original, nil
	}
	if r.node != nil {
		if err := r.node.allowChange(); err != nil {
			return nil, err
		}
	}
	if r.element != nil {
		elem, err := r.element.Reflected()
		if err != nil {
			return nil, err
		}
		switch r.original.Kind() {
		case reflect.Pointer:
			return reflect.PointerTo(elem), nil
		case reflect.Slice:
			return reflect.SliceOf(elem), nil
		case reflect.Array:
			return reflect.ArrayOf(r.original.Len(), elem), nil
		case reflect.Map:
			return reflect.MapOf(r.original.Key(), elem), nil
		}
	}
	fields := make([]RuntimeField, len(r.fields))
	for i, field := range r.fields {
		typ, err := field.shape.Reflected()
		if err != nil {
			return nil, err
		}
		fields[i] = RuntimeField{Name: field.metadata.Name, Tag: field.metadata.Tag, Type: typ, Anonymous: field.original.Anonymous, PkgPath: field.original.PkgPath}
	}
	result, err := (Runtime{}).Struct(fields)
	if err != nil {
		return nil, err
	}
	if result.Size() != r.original.Size() || result.Align() != r.original.Align() {
		return nil, fmt.Errorf("representation changed layout of %s", r.original)
	}
	for i := range fields {
		if result.Field(i).Offset != r.original.Field(i).Offset {
			return nil, fmt.Errorf("representation changed field offset in %s", r.original)
		}
	}
	return result, nil
}

// FieldShapeByIndex follows canonical indexes through pointer holders. Metadata
// names never change these storage indexes.
func (r *Representation) FieldShapeByIndex(index []int) *Representation {
	for _, i := range index {
		for r.original.Kind() == reflect.Pointer {
			r = r.element
		}
		r = r.fields[i].shape
	}
	return r
}

// Signature is deterministic metadata identity for generated schema names. It
// contains no instance values, callbacks or caller state. Cache owners should
// still namespace it by component/configuration and canonical type authority.
func (r *Representation) Signature() string {
	if r == nil {
		return ""
	}
	var result strings.Builder
	var write func(*Representation)
	write = func(node *Representation) {
		result.WriteString(strconv.Quote(node.original.PkgPath()))
		result.WriteString(strconv.Quote(node.original.String()))
		if !node.changed {
			return
		}
		result.WriteByte('{')
		if node.element != nil {
			write(node.element)
		}
		for _, field := range node.fields {
			result.WriteString(strconv.Quote(field.metadata.Name))
			result.WriteString(strconv.Quote(string(field.metadata.Tag)))
			write(field.shape)
		}
		result.WriteByte('}')
	}
	write(r)
	return result.String()
}

// Lookup follows a canonical Go field path through collection/pointer wrappers.
// It returns metadata for the storage field, independent of any public renaming.
func (r *Representation) Lookup(path string) (*Representation, error) {
	if r == nil {
		return nil, fmt.Errorf("representation is required")
	}
	if path == "" {
		return r, nil
	}
	for _, name := range strings.Split(path, ".") {
		for r.original.Kind() == reflect.Pointer || r.original.Kind() == reflect.Slice || r.original.Kind() == reflect.Array || r.original.Kind() == reflect.Map {
			if r.element == nil {
				return nil, fmt.Errorf("field path %s crosses an opaque representation", path)
			}
			r = r.element
		}
		if r.original.Kind() != reflect.Struct {
			return nil, fmt.Errorf("field path %s crosses %s", path, r.original)
		}
		field, ok := r.original.FieldByName(name)
		if !ok {
			return nil, fmt.Errorf("field %s is missing in %s", name, r.original)
		}
		r = r.FieldShapeByIndex(field.Index)
	}
	return r, nil
}

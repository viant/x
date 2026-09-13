package shape

import (
	"fmt"
	"reflect"
)

// CloneOptions limits cloning for exact struct types. Types absent from Fields
// retain normal CloneValue behavior. A present type with no fields clones as its
// zero value; omitted fields are neither read nor traversed. Names identify
// direct exported Go fields, not promoted fields or serialized aliases.
// All occurrences of a selected type use the same selection, preserving aliases.
// The caller must not mutate Fields or its slices during CloneValue.
type CloneOptions struct {
	Fields map[reflect.Type][]string
}

// Select adds promoted or dot-separated exported field paths using linked shape
// metadata. Every traversed holder receives a direct selection; no entire
// embedded struct is copied merely to reach a leaf. Pointer holders are resolved
// by type only, so selection never dereferences or initializes source values.
// Private holders and paths through collections are unsupported. Changes are
// atomic on error. Selections are unioned per exact type across all paths/calls.
// With no paths, typ receives an empty selection unless it already has one.
func (o *CloneOptions) Select(typ reflect.Type, paths ...string) error {
	if o == nil {
		return fmt.Errorf("clone options are required")
	}
	typ = (Runtime{}).Indirect(typ)
	if typ == nil || typ.Kind() != reflect.Struct {
		return fmt.Errorf("clone selection requires a struct type, got %v", typ)
	}
	fields := make(map[reflect.Type][]string, len(o.Fields)+1)
	for key, names := range o.Fields {
		fields[key] = append([]string(nil), names...)
	}
	if _, exists := fields[typ]; !exists {
		fields[typ] = nil
	}
	for _, path := range paths {
		field, err := Linked(typ).StructField(path)
		if err != nil {
			return fmt.Errorf("clone selection %s.%s: %w", typ, path, err)
		}
		current := typ
		for _, index := range field.Index {
			current = (Runtime{}).Indirect(current)
			if current == nil || current.Kind() != reflect.Struct {
				return fmt.Errorf("clone selection %s.%s traverses a nonstruct holder", typ, path)
			}
			member := current.Field(index)
			if !member.IsExported() {
				return fmt.Errorf("clone selection %s.%s traverses private field %s", typ, path, member.Name)
			}
			found := false
			for _, name := range fields[current] {
				if name == member.Name {
					found = true
					break
				}
			}
			if !found {
				fields[current] = append(fields[current], member.Name)
			}
			current = member.Type
		}
	}
	staged := CloneOptions{Fields: fields}
	if err := (&valueCloner{}).configure([]CloneOptions{staged}); err != nil {
		return err
	}
	o.Fields = fields
	return nil
}

func (c *valueCloner) configure(options []CloneOptions) error {
	if len(options) > 1 {
		return fmt.Errorf("CloneValue accepts at most one CloneOptions value")
	}
	if len(options) == 0 {
		return nil
	}
	c.fields = make(map[reflect.Type]map[int]bool, len(options[0].Fields))
	for typ, names := range options[0].Fields {
		if typ == nil || typ.Kind() != reflect.Struct {
			return fmt.Errorf("clone field selection requires an exact struct type, got %v", typ)
		}
		if c.synchronization(typ.PkgPath()) {
			return fmt.Errorf("clone field selection cannot copy synchronization type %s", typ)
		}
		selected := make(map[int]bool, len(names))
		for _, name := range names {
			field, found := typ.FieldByName(name)
			if !found || len(field.Index) != 1 || !field.IsExported() {
				return fmt.Errorf("clone field selection %s.%s requires a direct exported field", typ, name)
			}
			selected[field.Index[0]] = true
		}
		c.fields[typ] = selected
	}
	return nil
}

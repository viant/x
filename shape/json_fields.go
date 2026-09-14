//go:build go1.24

// Copyright 2010 The Go Authors. All rights reserved.
// Copyright 2011 The Go Authors. All rights reserved.
// Adapted from Go 1.25.8 encoding/json.typeFields; see JSON_FIELDS_LICENSE.
package shape

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unicode"
)

// JSONField is one selected encoding/json v1 field. Selection follows JSON
// dominance, not Go selector visibility. Field retains the exact source type.
// OptionalHolder means an anonymous pointer ancestor may omit the field.
type JSONField struct {
	Field                                       Field
	Name                                        string
	Quoted, OmitEmpty, OmitZero, OptionalHolder bool
}

// MayOmit reports the standard encoder's omission policy. In particular,
// omitempty does not omit structs or nonzero-length fixed arrays. IsZero methods
// under omitzero remain author-controlled and therefore may omit a field.
func (f JSONField) MayOmit() bool {
	if f.OptionalHolder || f.OmitZero {
		return true
	}
	if !f.OmitEmpty {
		return false
	}
	t := f.Field.ReflectedType
	switch t.Kind() {
	case reflect.Array:
		return t.Len() == 0
	case reflect.Map, reflect.Slice, reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64, reflect.Interface, reflect.Pointer:
		return true
	}
	return false
}

type jsonCandidate struct {
	JSONField
	tagged bool
}
type jsonLevel struct {
	typ      reflect.Type
	index    []int
	optional bool
}

// JSONFields enumerates linked standard-library JSON fields in encoding order.
// It does not inspect values, invoke custom code, or construct runtime types.
func (t *Type) JSONFields() ([]JSONField, error) {
	if t == nil || t.descriptor == nil || t.descriptor.Type == nil {
		return nil, fmt.Errorf("JSON field selection requires a linked type")
	}
	root := (Runtime{}).Indirect(t.descriptor.Type)
	if root.Kind() != reflect.Struct {
		return nil, fmt.Errorf("JSON field selection requires a struct, got %s", root)
	}
	current := []jsonLevel{}
	next := []jsonLevel{{typ: root}}
	var count, nextCount map[reflect.Type]int
	visited := map[reflect.Type]bool{}
	var fields []jsonCandidate
	for len(next) > 0 {
		current, next = next, current[:0]
		count, nextCount = nextCount, map[reflect.Type]int{}
		for _, parent := range current {
			if visited[parent.typ] {
				continue
			}
			visited[parent.typ] = true
			for i := 0; i < parent.typ.NumField(); i++ {
				sf := parent.typ.Field(i)
				if sf.Anonymous {
					base := sf.Type
					if base.Kind() == reflect.Pointer {
						base = base.Elem()
					}
					if !sf.IsExported() && base.Kind() != reflect.Struct {
						continue
					}
				} else if !sf.IsExported() {
					continue
				}
				tag := sf.Tag.Get("json")
				if tag == "-" {
					continue
				}
				name, rawOptions, _ := strings.Cut(tag, ",")
				metadata := jsonTag(rawOptions)
				if !metadata.validName(name) {
					name = ""
				}
				index := append(append([]int(nil), parent.index...), i)
				ft := sf.Type
				if ft.Name() == "" && ft.Kind() == reflect.Pointer {
					ft = ft.Elem()
				}
				quoted := false
				if metadata.contains("string") {
					switch ft.Kind() {
					case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64, reflect.String:
						quoted = true
					}
				}
				if name != "" || !sf.Anonymous || ft.Kind() != reflect.Struct {
					tagged := name != ""
					if name == "" {
						name = sf.Name
					}
					field := jsonCandidate{JSONField: JSONField{
						Field: Field{Name: sf.Name, ReflectedType: sf.Type, Tag: sf.Tag, Index: index, Anonymous: sf.Anonymous, Exported: sf.IsExported(), PkgPath: sf.PkgPath},
						Name:  name, Quoted: quoted, OmitEmpty: metadata.contains("omitempty"), OmitZero: metadata.contains("omitzero"), OptionalHolder: parent.optional,
					}, tagged: tagged}
					fields = append(fields, field)
					if count[parent.typ] > 1 {
						fields = append(fields, field)
					}
					continue
				}
				nextCount[ft]++
				if nextCount[ft] == 1 {
					next = append(next, jsonLevel{typ: ft, index: index, optional: parent.optional || sf.Type.Kind() == reflect.Pointer})
				}
			}
		}
	}
	slices.SortFunc(fields, func(a, b jsonCandidate) int {
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		if c := cmp.Compare(len(a.Field.Index), len(b.Field.Index)); c != 0 {
			return c
		}
		if a.tagged != b.tagged {
			if a.tagged {
				return -1
			}
			return 1
		}
		return slices.Compare(a.Field.Index, b.Field.Index)
	})
	selected := make([]JSONField, 0, len(fields))
	for i := 0; i < len(fields); {
		j := i + 1
		for j < len(fields) && fields[j].Name == fields[i].Name {
			j++
		}
		first := fields[i]
		// Equal-depth fields with the same tag priority annihilate each other.
		if j == i+1 || len(first.Field.Index) != len(fields[i+1].Field.Index) || first.tagged != fields[i+1].tagged {
			selected = append(selected, first.JSONField)
		}
		i = j
	}
	slices.SortFunc(selected, func(a, b JSONField) int { return slices.Compare(a.Field.Index, b.Field.Index) })
	return selected, nil
}

type jsonTag string

func (t jsonTag) contains(want string) bool {
	remaining := string(t)
	for remaining != "" {
		var part string
		part, remaining, _ = strings.Cut(remaining, ",")
		if part == want {
			return true
		}
	}
	return false
}
func (jsonTag) validName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", r) {
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

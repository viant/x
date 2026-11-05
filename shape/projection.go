package shape

import (
	"fmt"
	"reflect"
)

// ValueProjection maps canonical values to a Name/Tag-only representation.
// It never mutates its source. Unchanged values are reused, not deep cloned.
// Compile once beside the effective type; do not share mutable values across
// callers merely because their immutable projection plan is shared.
type ValueProjection struct {
	source, target reflect.Type
	fields         []*ValueProjection
	elem           *ValueProjection
}

func CompileProjection(source, target reflect.Type) (*ValueProjection, error) {
	return compileProjection(source, target, map[[2]reflect.Type]bool{})
}

func compileProjection(source, target reflect.Type, active map[[2]reflect.Type]bool) (*ValueProjection, error) {
	if source == nil || target == nil {
		return nil, fmt.Errorf("projection types are required")
	}
	p := &ValueProjection{source: source, target: target}
	if source == target {
		return p, nil
	}
	if source.Kind() != target.Kind() {
		return nil, fmt.Errorf("projection changes storage kind: %s to %s", source, target)
	}
	pair := [2]reflect.Type{source, target}
	if active[pair] {
		return nil, fmt.Errorf("recursive type projection requires a codec adapter")
	}
	active[pair] = true
	defer delete(active, pair)
	for _, typ := range []reflect.Type{source, target} {
		for _, codec := range rewriteOpaqueInterfaces {
			if typ.Implements(codec) || reflect.PointerTo(typ).Implements(codec) {
				return nil, fmt.Errorf("custom encoded type %s requires a codec projection", typ)
			}
		}
	}
	var err error
	switch source.Kind() {
	case reflect.Struct:
		if source.NumField() != target.NumField() {
			return nil, fmt.Errorf("projection changes field count")
		}
		p.fields = make([]*ValueProjection, source.NumField())
		for i := range p.fields {
			a, b := source.Field(i), target.Field(i)
			if a.Anonymous != b.Anonymous || a.PkgPath != b.PkgPath || a.IsExported() != b.IsExported() {
				return nil, fmt.Errorf("projection changes field authority at %d", i)
			}
			if a.Anonymous && a.Name != b.Name {
				return nil, fmt.Errorf("projection renames embedded holder")
			}
			if !a.IsExported() && !source.ConvertibleTo(target) {
				return nil, fmt.Errorf("projection of private field %s requires a codec adapter", a.Name)
			}
			p.fields[i], err = compileProjection(a.Type, b.Type, active)
			if err != nil {
				return nil, err
			}
		}
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		if source.Kind() == reflect.Array && source.Len() != target.Len() {
			return nil, fmt.Errorf("projection changes array length")
		}
		if source.Kind() == reflect.Map && source.Key() != target.Key() {
			return nil, fmt.Errorf("projection changes map keys")
		}
		p.elem, err = compileProjection(source.Elem(), target.Elem(), active)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("projection changes leaf type: %s to %s", source, target)
	}
	return p, nil
}

// Apply accepts the exact source type. Retains nil pointers/collections, scalar
// values, unchanged named types and aliases among changed pointer/map nodes.
// Changed slices retain length/capacity and values throughout their capacity.
func (p *ValueProjection) Apply(value any) (any, error) {
	if p == nil {
		return nil, fmt.Errorf("projection is required")
	}
	if value == nil {
		return nil, nil
	}
	v := reflect.ValueOf(value)
	if v.Type() != p.source {
		return nil, fmt.Errorf("projection expects %s, got %s", p.source, v.Type())
	}
	if p.source == p.target {
		return value, nil
	}
	result, err := p.apply(v, make(map[projectionReference]reflect.Value))
	if err != nil {
		return nil, err
	}
	return result.Interface(), nil
}

type projectionReference struct {
	source, target reflect.Type
	address        uintptr
	capacity       int
}

func (p *ValueProjection) apply(v reflect.Value, seen map[projectionReference]reflect.Value) (reflect.Value, error) {
	if p.source == p.target {
		return v, nil
	}
	// Legal Go conversions preserve private storage and use the runtime's type
	// compatibility rules; no memory-layout assumptions or unsafe casts.
	if v.CanConvert(p.target) {
		return v.Convert(p.target), nil
	}
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.Zero(p.target), nil
		}
		key := projectionReference{source: p.source, target: p.target, address: v.Pointer()}
		if prior, ok := seen[key]; ok {
			return prior, nil
		}
		result := reflect.New(p.target.Elem())
		seen[key] = result
		child, err := p.elem.apply(v.Elem(), seen)
		if err != nil {
			return reflect.Value{}, err
		}
		result.Elem().Set(child)
		return result, nil
	case reflect.Struct:
		result := reflect.New(p.target).Elem()
		for i, field := range p.fields {
			child, err := field.apply(v.Field(i), seen)
			if err != nil {
				return reflect.Value{}, err
			}
			result.Field(i).Set(child)
		}
		return result, nil
	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(p.target), nil
		}
		key := projectionReference{source: p.source, target: p.target, address: v.Pointer(), capacity: v.Cap()}
		if prior, ok := seen[key]; ok {
			return prior.Slice(0, v.Len()), nil
		}
		result := reflect.MakeSlice(p.target, v.Cap(), v.Cap())
		seen[key] = result
		all := v.Slice(0, v.Cap())
		for i := 0; i < all.Len(); i++ {
			child, err := p.elem.apply(all.Index(i), seen)
			if err != nil {
				return reflect.Value{}, err
			}
			result.Index(i).Set(child)
		}
		return result.Slice(0, v.Len()), nil
	case reflect.Array:
		result := reflect.New(p.target).Elem()
		for i := 0; i < v.Len(); i++ {
			child, err := p.elem.apply(v.Index(i), seen)
			if err != nil {
				return reflect.Value{}, err
			}
			result.Index(i).Set(child)
		}
		return result, nil
	case reflect.Map:
		if v.IsNil() {
			return reflect.Zero(p.target), nil
		}
		key := projectionReference{source: p.source, target: p.target, address: uintptr(v.UnsafePointer())}
		if prior, ok := seen[key]; ok {
			return prior, nil
		}
		result := reflect.MakeMapWithSize(p.target, v.Len())
		seen[key] = result
		iter := v.MapRange()
		for iter.Next() {
			child, err := p.elem.apply(iter.Value(), seen)
			if err != nil {
				return reflect.Value{}, err
			}
			result.SetMapIndex(iter.Key(), child)
		}
		return result, nil
	}
	return reflect.Value{}, fmt.Errorf("unsupported value projection %s", v.Type())
}

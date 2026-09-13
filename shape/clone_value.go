package shape

import (
	"fmt"
	"reflect"
	"sort"
	"time"
)

// CloneValue detaches a value's mutable graph without changing its concrete
// type. Pointer/map cycles and shared references are retained. Slices preserve
// length/capacity and clone the full backing capacity, including hidden values.
// Identical slice starts/types/capacities may share their cloned backing store.
// Other overlapping allocations (including interior pointers and offset slice
// aliases) are rejected rather than silently changing graph relationships.
// Functions, channels, unsafe pointers, synchronization state and unexported
// mutable internals are unsupported. time.Time is a known immutable exception.
// On error no result is published and the source is never mutated.
// The caller must prevent concurrent mutation of the source during the call.
// An optional CloneOptions selects exported fields of exact struct types;
// omitted fields are left zero and are not traversed, including service fields.
func (Runtime) CloneValue(value any, options ...CloneOptions) (any, error) {
	cloner := valueCloner{references: map[valueReference]reflect.Value{}, mapTypes: map[uintptr]reflect.Type{}}
	if err := cloner.configure(options); err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	result, err := cloner.clone(reflect.ValueOf(value), "value")
	if err != nil {
		return nil, err
	}
	if err := cloner.validateRegions(); err != nil {
		return nil, err
	}
	return result.Interface(), nil
}

type valueReference struct {
	kind     reflect.Kind
	typ      reflect.Type
	address  uintptr
	capacity int
}
type valueRegion struct {
	start, end uintptr
	path       string
}
type valueCloner struct {
	references map[valueReference]reflect.Value
	mapTypes   map[uintptr]reflect.Type
	regions    []valueRegion
	fields     map[reflect.Type]map[int]bool
}

var immutableTimeType = reflect.TypeOf(time.Time{})

func (c *valueCloner) clone(value reflect.Value, path string) (reflect.Value, error) {
	typ := value.Type()
	if c.synchronization(typ.PkgPath()) {
		return reflect.Value{}, fmt.Errorf("clone %s: synchronization state %s is unsupported", path, typ)
	}
	selection, selected := c.fields[typ]
	if typ == immutableTimeType && !selected {
		return value, nil
	}
	switch value.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.String:
		return value, nil
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return reflect.Value{}, fmt.Errorf("clone %s: opaque mutable type %s is unsupported", path, typ)
	case reflect.Interface:
		result := reflect.New(typ).Elem()
		if value.IsNil() {
			return result, nil
		}
		child, err := c.clone(value.Elem(), path+".(value)")
		if err != nil {
			return reflect.Value{}, err
		}
		result.Set(child)
		return result, nil
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(typ), nil
		}
		key := valueReference{kind: reflect.Pointer, typ: typ, address: value.Pointer()}
		if previous, ok := c.references[key]; ok {
			return previous, nil
		}
		if err := c.region(key.address, typ.Elem().Size(), path); err != nil {
			return reflect.Value{}, err
		}
		result := reflect.New(typ.Elem())
		if result.Type() != typ {
			result = result.Convert(typ)
		}
		c.references[key] = result
		child, err := c.clone(value.Elem(), path+".*")
		if err != nil {
			return reflect.Value{}, err
		}
		result.Elem().Set(child)
		return result, nil
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(typ), nil
		}
		key := valueReference{kind: reflect.Slice, typ: typ, address: value.Pointer(), capacity: value.Cap()}
		if previous, ok := c.references[key]; ok {
			return previous.Slice(0, value.Len()), nil
		}
		if err := c.region(key.address, uintptr(value.Cap())*typ.Elem().Size(), path); err != nil {
			return reflect.Value{}, err
		}
		result := reflect.MakeSlice(typ, value.Cap(), value.Cap())
		c.references[key] = result
		full := value.Slice(0, value.Cap())
		for index := 0; index < full.Len(); index++ {
			child, err := c.clone(full.Index(index), fmt.Sprintf("%s[%d]", path, index))
			if err != nil {
				return reflect.Value{}, err
			}
			result.Index(index).Set(child)
		}
		return result.Slice(0, value.Len()), nil
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(typ), nil
		}
		address := uintptr(value.UnsafePointer())
		key := valueReference{kind: reflect.Map, typ: typ, address: address}
		if previous, ok := c.references[key]; ok {
			return previous, nil
		}
		if prior, ok := c.mapTypes[address]; ok && prior != typ {
			return reflect.Value{}, fmt.Errorf("clone %s: shared map with differing concrete types is unsupported", path)
		}
		c.mapTypes[address] = typ
		result := reflect.MakeMapWithSize(typ, value.Len())
		c.references[key] = result
		iter := value.MapRange()
		for iter.Next() {
			mapKey, err := c.clone(iter.Key(), path+"[key]")
			if err != nil {
				return reflect.Value{}, err
			}
			entry, err := c.clone(iter.Value(), path+"[value]")
			if err != nil {
				return reflect.Value{}, err
			}
			result.SetMapIndex(mapKey, entry)
		}
		return result, nil
	case reflect.Struct:
		for index := 0; index < typ.NumField(); index++ {
			if selected && !selection[index] {
				continue
			}
			field := typ.Field(index)
			if !field.IsExported() && (c.synchronization(field.PkgPath) || !c.immutable(field.Type)) {
				return reflect.Value{}, fmt.Errorf("clone %s.%s: unexported mutable field %s is unsupported", path, field.Name, field.Type)
			}
		}
		result := reflect.New(typ).Elem()
		if !selected {
			result.Set(value)
		}
		for index := 0; index < typ.NumField(); index++ {
			if selected && !selection[index] {
				continue
			}
			field := typ.Field(index)
			if !field.IsExported() {
				continue
			}
			child, err := c.clone(value.Field(index), path+"."+field.Name)
			if err != nil {
				return reflect.Value{}, err
			}
			result.Field(index).Set(child)
		}
		return result, nil
	case reflect.Array:
		result := reflect.New(typ).Elem()
		for index := 0; index < value.Len(); index++ {
			child, err := c.clone(value.Index(index), fmt.Sprintf("%s[%d]", path, index))
			if err != nil {
				return reflect.Value{}, err
			}
			result.Index(index).Set(child)
		}
		return result, nil
	default:
		return reflect.Value{}, fmt.Errorf("clone %s: type %s is unsupported", path, typ)
	}
}

func (c *valueCloner) region(start, size uintptr, path string) error {
	if size == 0 {
		return nil
	}
	end := start + size
	if end < start {
		return fmt.Errorf("clone %s: allocation range overflows", path)
	}
	c.regions = append(c.regions, valueRegion{start: start, end: end, path: path})
	return nil
}

func (c *valueCloner) validateRegions() error {
	sort.Slice(c.regions, func(i, j int) bool { return c.regions[i].start < c.regions[j].start })
	for index := 1; index < len(c.regions); index++ {
		previous, current := c.regions[index-1], c.regions[index]
		if current.start < previous.end {
			return fmt.Errorf("clone %s: overlapping/interior allocation with %s is unsupported", current.path, previous.path)
		}
	}
	return nil
}

func (c *valueCloner) immutable(typ reflect.Type) bool {
	// Unexported values are copied wholesale only when doing so cannot bypass
	// a selected-field policy nested inside them.
	if _, selected := c.fields[typ]; selected {
		return false
	}
	if typ == immutableTimeType {
		return true
	}
	if c.synchronization(typ.PkgPath()) {
		return false
	}
	switch typ.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.String:
		return true
	case reflect.Array:
		return c.immutable(typ.Elem())
	case reflect.Struct:
		for index := 0; index < typ.NumField(); index++ {
			field := typ.Field(index)
			if c.synchronization(field.PkgPath) || !c.immutable(field.Type) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (*valueCloner) synchronization(packagePath string) bool {
	switch packagePath {
	case "sync", "sync/atomic", "internal/sync", "internal/runtime/atomic", "runtime/internal/atomic":
		return true
	default:
		return false
	}
}

package xtype

import (
	"errors"
	"reflect"
)

var ErrUnnamedRecursion = errors.New("xtype: unnamed recursive composite is unsupported")

func FromReflect(t reflect.Type) (Node, error) {
	if t == nil {
		return nil, nil
	}
	namedMemo := map[string]*Named{}
	visiting := map[reflect.Type]bool{}
	return fromReflectType(t, namedMemo, visiting)
}

func fromReflectTypeIndirect(t reflect.Type, namedMemo map[string]*Named, visiting map[reflect.Type]bool) (Node, error) {
	return fromReflectType(t, namedMemo, visiting)
}

func fromReflectType(t reflect.Type, namedMemo map[string]*Named, visiting map[reflect.Type]bool) (Node, error) {
	if t == nil {
		return nil, nil
	}
	if t.Name() != "" {
		key := t.PkgPath() + "." + t.Name()
		if n, ok := namedMemo[key]; ok {
			return n, nil
		}
		named := &Named{PkgPath: t.PkgPath(), Name: t.Name()}
		namedMemo[key] = named
		u, err := fromReflectTypeIndirect(t, namedMemo, visiting)
		if err != nil {
			return nil, err
		}
		named.Underlying = u
		return named, nil
	}
	if visiting[t] {
		return nil, ErrUnnamedRecursion
	}
	visiting[t] = true
	defer delete(visiting, t)
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.String:
		return &Basic{Name: t.Name(), PkgPath: ""}, nil
	case reflect.Ptr:
		e, err := fromReflectType(t.Elem(), namedMemo, visiting)
		if err != nil {
			return nil, err
		}
		return &Pointer{Elem: e}, nil
	case reflect.Slice:
		e, err := fromReflectType(t.Elem(), namedMemo, visiting)
		if err != nil {
			return nil, err
		}
		return &Slice{Elem: e}, nil
	case reflect.Array:
		e, err := fromReflectType(t.Elem(), namedMemo, visiting)
		if err != nil {
			return nil, err
		}
		return &Array{Len: t.Len(), Elem: e}, nil
	case reflect.Map:
		k, err := fromReflectType(t.Key(), namedMemo, visiting)
		if err != nil {
			return nil, err
		}
		v, err := fromReflectType(t.Elem(), namedMemo, visiting)
		if err != nil {
			return nil, err
		}
		return &Map{Key: k, Elem: v}, nil
	case reflect.Chan:
		e, err := fromReflectType(t.Elem(), namedMemo, visiting)
		if err != nil {
			return nil, err
		}
		return &Chan{Dir: int(t.ChanDir()), Elem: e}, nil
	case reflect.Func:
		fn := Func{Variadic: t.IsVariadic()}
		fn.Params = make([]Field, t.NumIn())
		for i := 0; i < t.NumIn(); i++ {
			pt, err := fromReflectType(t.In(i), namedMemo, visiting)
			if err != nil {
				return nil, err
			}
			fn.Params[i] = Field{Type: pt}
		}
		fn.Results = make([]Field, t.NumOut())
		for i := 0; i < t.NumOut(); i++ {
			rt, err := fromReflectType(t.Out(i), namedMemo, visiting)
			if err != nil {
				return nil, err
			}
			fn.Results[i] = Field{Type: rt}
		}
		return &fn, nil
	case reflect.Interface:
		iface := Interface{Methods: make([]Method, 0, t.NumMethod())}
		for i := 0; i < t.NumMethod(); i++ {
			m := t.Method(i)
			if m.PkgPath != "" {
				continue
			}
			mtNode, err := fromReflectType(m.Type, namedMemo, visiting)
			if err != nil {
				return nil, err
			}
			mt, ok := mtNode.(*Func)
			if !ok {
				mt = &Func{}
			}
			iface.Methods = append(iface.Methods, Method{Name: m.Name, Type: *mt})
		}
		return &iface, nil
	case reflect.Struct:
		fields := make([]Field, 0, t.NumField())
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			ft, err := fromReflectType(f.Type, namedMemo, visiting)
			if err != nil {
				return nil, err
			}
			fields = append(fields, Field{Name: f.Name, Type: ft, Tag: string(f.Tag), Embedded: f.Anonymous})
		}
		return &Struct{Fields: fields}, nil
	default:
		return nil, nil
	}
}

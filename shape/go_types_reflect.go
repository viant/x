package shape

import (
	"fmt"
	"go/token"
	"go/types"
	"reflect"
)

func (b *goTypeAuthority) reflected(typ reflect.Type) (types.Type, error) {
	if typ == nil {
		return nil, fmt.Errorf("linked type is required")
	}
	if cached := b.linked[typ]; cached != nil {
		return cached, nil
	}
	if typ.Name() != "" {
		if typ.PkgPath() == "" {
			if object := types.Universe.Lookup(typ.Name()); object != nil {
				return object.Type(), nil
			}
		}
		identity, err := (Resolver{Qualifier: func(location, suggested string) string { return location }}).Expression(typ)
		if err != nil {
			return nil, err
		}
		if prior := b.named[identity]; prior != nil {
			b.linked[typ] = prior
			return prior, nil
		}
		named := types.NewNamed(types.NewTypeName(token.NoPos, b.pkg(typ.PkgPath()), typ.Name(), nil), nil, nil)
		b.linked[typ] = named
		b.named[identity] = named
		underlying, err := b.reflectedUnderlying(typ)
		if err != nil {
			return nil, err
		}
		named.SetUnderlying(underlying)
		if typ.Kind() != reflect.Interface {
			pointer := reflect.PointerTo(typ)
			for i := 0; i < pointer.NumMethod(); i++ {
				method := pointer.Method(i)
				var receiver types.Type = named
				if _, ok := typ.MethodByName(method.Name); !ok {
					receiver = types.NewPointer(named)
				}
				signature, err := b.reflectedSignature(method.Type, 1, types.NewVar(token.NoPos, b.pkg(typ.PkgPath()), "", receiver))
				if err != nil {
					return nil, err
				}
				named.AddMethod(types.NewFunc(token.NoPos, b.pkg(typ.PkgPath()), method.Name, signature))
			}
		}
		return named, nil
	}
	result, err := b.reflectedUnderlying(typ)
	if err == nil {
		b.linked[typ] = result
	}
	return result, err
}

func (b *goTypeAuthority) reflectedUnderlying(typ reflect.Type) (types.Type, error) {
	basics := map[reflect.Kind]types.BasicKind{reflect.Bool: types.Bool, reflect.Int: types.Int, reflect.Int8: types.Int8, reflect.Int16: types.Int16, reflect.Int32: types.Int32, reflect.Int64: types.Int64, reflect.Uint: types.Uint, reflect.Uint8: types.Uint8, reflect.Uint16: types.Uint16, reflect.Uint32: types.Uint32, reflect.Uint64: types.Uint64, reflect.Uintptr: types.Uintptr, reflect.Float32: types.Float32, reflect.Float64: types.Float64, reflect.Complex64: types.Complex64, reflect.Complex128: types.Complex128, reflect.String: types.String, reflect.UnsafePointer: types.UnsafePointer}
	if kind, ok := basics[typ.Kind()]; ok {
		return types.Typ[kind], nil
	}
	switch typ.Kind() {
	case reflect.Pointer:
		element, err := b.reflected(typ.Elem())
		if err != nil {
			return nil, err
		}
		return types.NewPointer(element), nil
	case reflect.Slice:
		element, err := b.reflected(typ.Elem())
		if err != nil {
			return nil, err
		}
		return types.NewSlice(element), nil
	case reflect.Array:
		element, err := b.reflected(typ.Elem())
		if err != nil {
			return nil, err
		}
		return types.NewArray(element, int64(typ.Len())), nil
	case reflect.Map:
		key, err := b.reflected(typ.Key())
		if err != nil {
			return nil, err
		}
		element, err := b.reflected(typ.Elem())
		if err != nil {
			return nil, err
		}
		return types.NewMap(key, element), nil
	case reflect.Chan:
		element, err := b.reflected(typ.Elem())
		if err != nil {
			return nil, err
		}
		direction := types.SendRecv
		if typ.ChanDir() == reflect.SendDir {
			direction = types.SendOnly
		}
		if typ.ChanDir() == reflect.RecvDir {
			direction = types.RecvOnly
		}
		return types.NewChan(direction, element), nil
	case reflect.Func:
		return b.reflectedSignature(typ, 0, nil)
	case reflect.Interface:
		methods := []*types.Func{}
		for i := 0; i < typ.NumMethod(); i++ {
			method := typ.Method(i)
			signature, err := b.reflectedSignature(method.Type, 0, nil)
			if err != nil {
				return nil, err
			}
			methods = append(methods, types.NewFunc(token.NoPos, b.pkg(method.PkgPath), method.Name, signature))
		}
		return types.NewInterfaceType(methods, nil).Complete(), nil
	case reflect.Struct:
		fields := []*types.Var{}
		tags := []string{}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			value, err := b.reflected(field.Type)
			if err != nil {
				return nil, err
			}
			fields = append(fields, types.NewField(token.NoPos, b.pkg(field.PkgPath), field.Name, value, field.Anonymous))
			tags = append(tags, string(field.Tag))
		}
		return types.NewStruct(fields, tags), nil
	}
	return nil, fmt.Errorf("linked type %s has unsupported Go type kind", typ)
}
func (b *goTypeAuthority) reflectedSignature(typ reflect.Type, first int, receiver *types.Var) (*types.Signature, error) {
	parameters := []*types.Var{}
	results := []*types.Var{}
	for i := first; i < typ.NumIn(); i++ {
		value, err := b.reflected(typ.In(i))
		if err != nil {
			return nil, err
		}
		parameters = append(parameters, types.NewVar(token.NoPos, nil, "", value))
	}
	for i := 0; i < typ.NumOut(); i++ {
		value, err := b.reflected(typ.Out(i))
		if err != nil {
			return nil, err
		}
		results = append(results, types.NewVar(token.NoPos, nil, "", value))
	}
	return types.NewSignatureType(receiver, nil, nil, types.NewTuple(parameters...), types.NewTuple(results...), typ.IsVariadic()), nil
}

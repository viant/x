package shape

import "reflect"

func fieldPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "." + name
}

func elementPath(parent string, kind reflect.Kind) string {
	switch kind {
	case reflect.Slice, reflect.Array:
		return parent + "[]"
	case reflect.Map:
		return parent + "{}"
	default:
		return parent
	}
}

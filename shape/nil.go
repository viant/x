package shape

import "reflect"

// IsNil reports nil interface values and typed nil values of every nil-capable
// Go kind. It does not dereference or invoke the supplied value.
func (Runtime) IsNil(value any) bool {
	if value == nil {
		return true
	}
	actual := reflect.ValueOf(value)
	switch actual.Kind() {
	case reflect.Interface, reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return actual.IsNil()
	}
	return false
}

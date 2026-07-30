// Package nilcheck safely detects nil values hidden inside interfaces.
package nilcheck

import "reflect"

// IsNil reports whether value is nil without calling reflect.Value.IsNil for
// kinds that cannot be nil.
func IsNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

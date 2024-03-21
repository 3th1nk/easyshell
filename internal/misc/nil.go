package misc

import "reflect"

func IsNil(v interface{}) bool {
	if v == nil {
		return true
	}

	switch reflect.TypeOf(v).Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.Interface, reflect.Slice:
		return reflect.ValueOf(v).IsNil()
	default:
		return false
	}
}

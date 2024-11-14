package tuple

import (
	"fmt"
	"reflect"
)

func SetToStruct(tuple interface{}, dst interface{}, offset int) error {
	tupleValue := reflect.ValueOf(tuple)
	if tupleValue.Kind() == reflect.Ptr {
		tupleValue = tupleValue.Elem()
	}
	if tupleValue.Kind() != reflect.Struct {
		return fmt.Errorf("tuple must be a struct, got %v", tupleValue.Kind())
	}
	dstValue := reflect.ValueOf(dst)
	if dstValue.Kind() != reflect.Ptr {
		return fmt.Errorf("destination must be a pointer, got %v", dstValue.Kind())
	}
	dstValue = dstValue.Elem()
	if dstValue.Kind() != reflect.Struct {
		return fmt.Errorf("destination must be a pointer to struct, got pointer to %v", dstValue.Kind())
	}
	tupleFields := tupleValue.NumField()
	dstFields := dstValue.NumField()
	for i := 0; i < tupleFields; i++ {
		if i+offset < 0 || i+offset >= dstFields {
			continue
		}
		tupleField := tupleValue.Field(i)
		dstField := dstValue.Field(i + offset)
		if !dstField.CanSet() {
			return fmt.Errorf("field %d in destination struct is not settable", i)
		}
		if !tupleField.Type().AssignableTo(dstField.Type()) {
			return fmt.Errorf("field %d type mismatch: cannot assign %v to %v",
				i, tupleField.Type(), dstField.Type())
		}
		dstField.Set(tupleField)
	}

	return nil
}

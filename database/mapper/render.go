package mapper

import (
	"encoding/json"
	"fmt"
	"golang.org/x/exp/constraints"
	"reflect"
	"strings"
)

func Map[S any, T any](source []S, mapper func(S) T) []T {
	result := make([]T, len(source))
	for i, s := range source {
		result[i] = mapper(s)
	}
	return result
}

func MapStruct[S any, T any](source *S) *T {
	bytes, err := json.Marshal(source)
	if err != nil {
		return nil
	}
	var target T
	err = json.Unmarshal(bytes, &target)
	if err != nil {
		return nil
	}
	return &target
}

func SetFields[S any, T any](source S, target T, ignoreZero bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	sourceValue := reflect.ValueOf(source)
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		err = fmt.Errorf("target must be a pointer")
		return
	}
	targetValue = targetValue.Elem()
	sourceType := sourceValue.Type()
	for i := 0; i < sourceValue.NumField(); i++ {
		field := sourceType.Field(i)
		fieldValue := sourceValue.Field(i)
		setterName := "Set" + strings.Title(field.Name)
		method := targetValue.Addr().MethodByName(setterName)
		if method.IsValid() {
			args := []reflect.Value{fieldValue}
			if ignoreZero && fieldValue.IsZero() {
				continue
			}
			method.Call(args)
		}
	}
	return
}

func Page[P constraints.Integer](total, pageSize, defaultSize P) P {
	if total == 0 {
		return 0
	}
	if pageSize == 0 {
		pageSize = defaultSize
	}
	if pageSize == 0 {
		return total
	}
	page := total / pageSize
	if total%pageSize != 0 {
		page++
	}
	return page
}

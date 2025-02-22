package mapper

import (
	"encoding/json"
	"fmt"
	"golang.org/x/exp/constraints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"reflect"
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
	for sourceValue.Kind() == reflect.Ptr {
		sourceValue = sourceValue.Elem()
	}
	targetValue = targetValue.Elem()
	sourceType := sourceValue.Type()
	for i := 0; i < sourceValue.NumField(); i++ {
		field := sourceType.Field(i)
		fieldValue := sourceValue.Field(i)
		setterName := "Set" + cases.Title(language.Und, cases.NoLower).String(field.Name)
		//setterName := "Set" + cases.Title(field.Name).String()
		method := targetValue.Addr().MethodByName(setterName)
		if method.IsValid() {
			if method.Type().NumIn() != 1 {
				err = fmt.Errorf("method %s must have one parameter", setterName)
				return
			}
			methodParamType := method.Type().In(0)
			if method.Type().In(0) != fieldValue.Type() {
				if fieldValue.Type().ConvertibleTo(methodParamType) {
					fieldValue = fieldValue.Convert(methodParamType)
				} else {
					err = fmt.Errorf("method %s parameter type must be %s, but got %s", setterName, methodParamType, fieldValue.Type())
					return
				}
			}
			if ignoreZero && fieldValue.IsZero() {
				continue
			}
			args := []reflect.Value{fieldValue}
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

package bind

import (
	"fmt"
	"reflect"
	"unicode"
)

func getPublicFields(obj interface{}, keyMapper func(s string) string) ([]string, map[string]reflect.StructField) {
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil, nil
	}
	typ := val.Type()
	keys := make([]string, 0)
	fields := make(map[string]reflect.StructField)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if unicode.IsUpper(rune(field.Name[0])) && !field.Anonymous {
			k := field.Name
			if keyMapper != nil {
				k = keyMapper(k)
			}
			keys = append(keys, k)
			fields[k] = field
		}
	}
	return keys, fields
}

func getPublicMethods(obj interface{}, keyMapper func(s string) string) ([]string, map[string]reflect.Method) {
	typ := reflect.TypeOf(obj)

	if typ == nil || (typ.Kind() != reflect.Struct && (typ.Kind() != reflect.Ptr || typ.Elem().Kind() != reflect.Struct)) {
		return nil, nil
	}

	keys := make([]string, 0)
	methods := make(map[string]reflect.Method)

	structType := typ
	ptrType := typ
	if typ.Kind() == reflect.Ptr {
		structType = typ.Elem()
	} else {
		ptrType = reflect.PointerTo(typ)
	}

	for i := 0; i < structType.NumMethod(); i++ {
		method := structType.Method(i)
		if unicode.IsUpper(rune(method.Name[0])) {
			k := method.Name
			if keyMapper != nil {
				k = keyMapper(k)
			}
			keys = append(keys, k)
			methods[k] = method
		}
	}

	for i := 0; i < ptrType.NumMethod(); i++ {
		method := ptrType.Method(i)
		k := method.Name
		if keyMapper != nil {
			k = keyMapper(k)
		}
		if _, exists := methods[k]; !exists && unicode.IsUpper(rune(method.Name[0])) {
			keys = append(keys, k)
			methods[k] = method
		}
	}

	return keys, methods
}

func getStructName(value any) string {
	v := reflect.ValueOf(value)
	t := reflect.TypeOf(value)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		return t.Name()
	}
	return "Unknown"
}

func genZeroCheck(sourceName string, field reflect.StructField) string {
	if field.Type.Kind() == reflect.Ptr {
		return fmt.Sprintf("%s.%s == nil", sourceName, field.Name)
	}
	switch field.Type.Kind() {
	case reflect.String:
		return fmt.Sprintf("%s.%s == \"\"", sourceName, field.Name)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%s.%s == 0", sourceName, field.Name)
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%s.%s == 0.0", sourceName, field.Name)
	case reflect.Bool:
		return fmt.Sprintf("!%s.%s", sourceName, field.Name)
	case reflect.Slice:
		return fmt.Sprintf("%s.%s == nil", sourceName, field.Name)
	default:
		return fmt.Sprintf("reflect.ValueOf(%s.%s).IsZero()", sourceName, field.Name)
	}
}

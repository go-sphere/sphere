package bind

import (
	"fmt"
	"path"
	"reflect"
	"sort"
	"strings"
)

// getPublicFields extracts all public (exported) fields from a struct using reflection.
// It returns field names transformed by the keyMapper function and a map of field metadata.
// Fields that are not exported or are anonymous are excluded from the result.
func getPublicFields(obj interface{}, keyMapper func(s string) string) ([]string, map[string]reflect.StructField) {
	val := reflect.ValueOf(obj)
	for val.Kind() == reflect.Ptr {
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
		if !field.IsExported() || field.Anonymous {
			continue
		}
		k := field.Name
		if keyMapper != nil {
			k = keyMapper(k)
		}
		keys = append(keys, k)
		fields[k] = field
	}
	return keys, fields
}

// getPublicMethods extracts all public methods from a type using reflection.
// It returns method names transformed by the keyMapper function and a map of method metadata.
// Interface types are not supported and will return nil values.
func getPublicMethods(obj interface{}, keyMapper func(s string) string) ([]string, map[string]reflect.Method) {
	typ := reflect.TypeOf(obj)
	if typ == nil {
		return nil, nil
	}
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() == reflect.Interface {
		return nil, nil
	}

	ptrType := reflect.PointerTo(typ)
	numMethod := ptrType.NumMethod()
	keys := make([]string, 0, numMethod)
	methods := make(map[string]reflect.Method, numMethod)

	for i := 0; i < numMethod; i++ {
		method := ptrType.Method(i)

		k := method.Name
		if keyMapper != nil {
			k = keyMapper(k)
		}

		keys = append(keys, k)
		methods[k] = method
	}
	return keys, methods
}

// getStructName extracts the struct type name from any value using reflection.
// It handles pointer types by dereferencing them to get the underlying struct name.
// Returns "Unknown" if the value is not a struct type.
func getStructName(value any) string {
	v := reflect.ValueOf(value)
	t := reflect.TypeOf(value)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		return t.Name()
	}
	return "Unknown"
}

// genZeroCheck generates Go code that checks if a struct field contains its zero value.
// The generated condition varies based on the field's type (pointer, string, numeric, etc.).
// This is used in template generation for conditional field processing.
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
	case reflect.Slice, reflect.Array, reflect.Map, reflect.Struct:
		return fmt.Sprintf("%s.%s == nil", sourceName, field.Name)
	default:
		return fmt.Sprintf("reflect.ValueOf(%s.%s).IsZero()", sourceName, field.Name)
	}
}

// genNotZeroCheck generates Go code that checks if a struct field contains a non-zero value.
// This is the inverse of genZeroCheck and is used for template generation
// to create conditions that verify field values are set.
func genNotZeroCheck(sourceName string, field reflect.StructField) string {
	if field.Type.Kind() == reflect.Ptr {
		return fmt.Sprintf("%s.%s != nil", sourceName, field.Name)
	}
	switch field.Type.Kind() {
	case reflect.String:
		return fmt.Sprintf("%s.%s != \"\"", sourceName, field.Name)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%s.%s != 0", sourceName, field.Name)
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%s.%s != 0.0", sourceName, field.Name)
	case reflect.Bool:
		return fmt.Sprintf("%s.%s", sourceName, field.Name)
	case reflect.Slice, reflect.Array, reflect.Map, reflect.Struct:
		return fmt.Sprintf("%s.%s != nil", sourceName, field.Name)
	default:
		return fmt.Sprintf("!reflect.ValueOf(%s.%s).IsZero()", sourceName, field.Name)
	}
}

func typeName(val any) string {
	value := reflect.ValueOf(val)
	for value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	typeOf := value.Type()
	return typeOf.Name()
}

func packagePath(val any) string {
	value := reflect.ValueOf(val)
	for value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return ""
	}
	typeOf := value.Type()
	return typeOf.PkgPath()
}

// packageName extracts the package name from a struct value's type information.
// It returns the first part of the fully qualified type name before the dot.
// Returns empty string if the value is not a struct or has no package qualifier.
func packageName(val any) string {
	value := reflect.ValueOf(val)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return ""
	}
	typeOf := value.Type()
	fullName := typeOf.String()
	if !strings.Contains(fullName, ".") {
		return ""
	}
	parts := strings.Split(fullName, ".")
	return parts[0]
}

// PackageImport extracts both the package path and package name from a struct value.
// It returns a 2-element array containing the full import path and the package name.
// This is used for generating proper import statements in code generation.
func PackageImport(val any) [2]string {
	pkgName := packageName(val)
	pkgPath := packagePath(val)
	return [2]string{
		pkgPath,
		pkgName,
	}
}

// compressedImports removes duplicate import entries and sorts them for consistent output.
// It also optimizes import aliases by removing redundant package names when they match
// the directory name. This is used in code generation to create clean import statements.
func compressedImports(extraImports [][2]string) [][2]string {
	seen := make(map[[2]string]bool)
	result := make([][2]string, 0, len(extraImports))
	for _, item := range extraImports {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i][0] == result[j][0] {
			return result[i][1] < result[j][1]
		}
		return result[i][0] < result[j][0]
	})
	for i, item := range result {
		_, name := path.Split(item[0])
		if item[1] == name {
			result[i][1] = ""
		}
	}
	return result
}

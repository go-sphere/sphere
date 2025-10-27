package bind

import (
	"fmt"
	"path"
	"reflect"
	"sort"
	"strings"
)

// Reflection Utilities

// indirectValue recursively dereferences pointers until a non-pointer value is reached.
// It returns the final dereferenced reflect.Value.
func indirectValue(value reflect.Value) reflect.Value {
	for value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	return value
}

// Type Information Extraction

// typeName returns the name of the most deeply dereferenced type of the given value.
// It handles pointer indirections automatically using reflection.
func typeName(value any) string {
	return indirectValue(reflect.ValueOf(value)).Type().Name()
}

// extractTypeName returns the simple type name without package prefix.
func extractTypeName(val any) string {
	value := indirectValue(reflect.ValueOf(val))
	typeOf := value.Type()
	return typeOf.Name()
}

// extractPackagePath returns the full import path of the package containing the type.
func extractPackagePath(val any) string {
	value := indirectValue(reflect.ValueOf(val))
	typeOf := value.Type()
	return typeOf.PkgPath()
}

// extractPackageName extracts the package name from a struct value's type information.
// It returns the first part of the fully qualified type name before the dot.
// Returns empty string if the value is not a struct or has no package qualifier.
func extractPackageName(val any) string {
	value := indirectValue(reflect.ValueOf(val))
	typeOf := value.Type()
	fullName := typeOf.String()
	if !strings.Contains(fullName, ".") {
		return ""
	}
	parts := strings.Split(fullName, ".")
	return parts[0]
}

// Struct and Method Inspection

// extractPublicFields extracts all public (exported) fields from a struct using reflection.
// It returns field names transformed by the keyMapper function and a map of field metadata.
// Fields that are not exported or are anonymous are excluded from the result.
func extractPublicFields(obj interface{}, keyMapper func(s string) string) ([]string, map[string]reflect.StructField) {
	if obj == nil {
		return nil, nil
	}
	val := indirectValue(reflect.ValueOf(obj))
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

// extractPublicMethods extracts all public methods from a type using reflection.
// It returns method names transformed by the keyMapper function and a map of method metadata.
// Interface types are not supported and will return nil values.
func extractPublicMethods(obj any, keyMapper func(string) string) ([]string, map[string]reflect.Method) {
	if obj == nil {
		return nil, nil
	}
	t := reflect.TypeOf(obj)
	if t.Kind() == reflect.Interface {
		return nil, nil
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	types := []reflect.Type{t, reflect.PointerTo(t)}
	keys := make([]string, 0)
	methods := make(map[string]reflect.Method)
	seen := make(map[string]struct{})

	for _, typ := range types {
		for i := 0; i < typ.NumMethod(); i++ {
			m := typ.Method(i)
			if !m.IsExported() {
				continue
			}
			name := m.Name
			if keyMapper != nil {
				name = keyMapper(name)
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			keys = append(keys, name)
			methods[name] = m
		}
	}
	return keys, methods
}

// Code Generation Helpers

// generateZeroCheckExpr generates Go code that checks if a struct field contains its zero value.
// The generated condition varies based on the field's type (pointer, string, numeric, etc.).
// This is used in template generation for conditional field processing.
func generateZeroCheckExpr(sourceName string, field reflect.StructField) string {
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

// generateNonZeroCheckExpr generates Go code that checks if a struct field contains a non-zero value.
// This is the inverse of generateZeroCheckExpr and is used for template generation
// to create conditions that verify field values are set.
func generateNonZeroCheckExpr(sourceName string, field reflect.StructField) string {
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

// Package Information

// extractPackageImport extracts both the package path and package name from a struct value.
// It returns a 2-element array containing the full import path and the package name.
// This is used for generating proper import statements in code generation.
func extractPackageImport(val any) [2]string {
	pkgName := extractPackageName(val)
	pkgPath := extractPackagePath(val)
	return [2]string{
		pkgPath,
		pkgName,
	}
}

// deduplicateImports removes duplicate import entries and sorts them for consistent output.
// It also optimizes import aliases by removing redundant package names when they match
// the directory name. This is used in code generation to create clean import statements.
func deduplicateImports(extraImports [][2]string) [][2]string {
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

package bind

import (
	_ "embed"
	"fmt"
	"reflect"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"
)

//go:embed func.tmpl
var genBindFuncTemplate string

// GenFuncConf holds configuration for generating binding functions between different data structures.
// It's commonly used for generating code that converts between ORM entities and Protocol Buffer messages.
type GenFuncConf struct {
	source        any      // ent entity, e.g. ent.Example
	target        any      // protobuf entity, e.g. entpb.Example
	action        any      // ent operation, e.g. ent.ExampleCreate, ent.ExampleUpdateOne
	IgnoreFields  []string // fields to ignore, e.g.  example.FieldID, example.FieldCreatedAt
	SourcePkgName string   // package name of source, e.g. "ent"
	TargetPkgName string   // package name of target, e.g. "entpb"
}

// NewGenFuncConf creates a new configuration for binding function generation.
// It automatically determines package names from the provided source and target types.
func NewGenFuncConf(source, target, action any) *GenFuncConf {
	return &GenFuncConf{
		source:        source,
		target:        target,
		action:        action,
		IgnoreFields:  nil,
		SourcePkgName: extractPackageName(source),
		TargetPkgName: extractPackageName(target),
	}
}

// WithSourcePkgName sets a custom package name for the source type.
// Returns the modified configuration for method chaining.
func (c *GenFuncConf) WithSourcePkgName(pkgName string) *GenFuncConf {
	c.SourcePkgName = pkgName
	return c
}

// WithTargetPkgName sets a custom package name for the target type.
// Returns the modified configuration for method chaining.
func (c *GenFuncConf) WithTargetPkgName(pkgName string) *GenFuncConf {
	c.TargetPkgName = pkgName
	return c
}

// WithIgnoreFields specifies field names that should be ignored during binding generation.
// Returns the modified configuration for method chaining.
func (c *GenFuncConf) WithIgnoreFields(fields ...string) *GenFuncConf {
	c.IgnoreFields = fields
	return c
}

// GenBindFunc generates Go code for binding functions based on the provided configuration.
// It creates functions that can convert between source and target types using reflection
// to analyze field mappings and generate appropriate setter calls.
// Returns the generated Go code as a string or an error if generation fails.
func GenBindFunc(conf *GenFuncConf) (string, error) {
	actionName := typeName(conf.action)
	sourceName := typeName(conf.source)
	targetName := typeName(conf.target)
	funcName := strings.Replace(actionName, sourceName, "", 1) + sourceName

	keys, sourceFields := extractPublicFields(conf.source, strcase.ToSnake)
	_, targetFields := extractPublicFields(conf.target, strcase.ToSnake)
	_, actionMethods := extractPublicMethods(conf.action, strcase.ToSnake)

	context := bindContext{
		SourcePkgName: conf.SourcePkgName,
		TargetPkgName: conf.TargetPkgName,

		ActionName: actionName,
		SourceName: sourceName,
		TargetName: targetName,
		FuncName:   funcName,
		Fields:     make([]fieldContext, 0),
	}

	ignoreFields := make(map[string]bool, len(conf.IgnoreFields))
	for _, field := range conf.IgnoreFields {
		ignoreFields[strings.ToLower(field)] = true
	}
	table := typeName(conf.source)

	for _, n := range keys {
		if ignoreFields[n] {
			continue
		}
		sourceField, ok := sourceFields[n] // ent.Example
		if !ok {
			continue
		}
		targetField, ok := targetFields[n] // entpb.Example
		if !ok {
			continue
		}

		setter, hasSetter := actionMethods[strcase.ToSnake(fmt.Sprintf("Set%s", sourceField.Name))]
		if !hasSetter {
			continue
		}
		settNillable, hasSettNillable := actionMethods[strcase.ToSnake(fmt.Sprintf("SetNillable%s", sourceField.Name))]
		clearOnNil, hasClearOnNil := actionMethods[strcase.ToSnake(fmt.Sprintf("Clear%s", sourceField.Name))]
		targetFieldIsPtr := targetField.Type.Kind() == reflect.Ptr

		field := fieldContext{
			FieldKeyPath: fmt.Sprintf("%s.Field%s", strings.ToLower(table), sourceField.Name),

			TargetField: targetField,
			SourceField: sourceField,

			SetterFuncName:       setter.Name,
			SettNillableFuncName: settNillable.Name,
			ClearOnNilFuncName:   clearOnNil.Name,

			CanSettNillable:        hasSettNillable,
			CanClearOnNil:          hasClearOnNil,
			TargetFieldIsPtr:       targetFieldIsPtr,
			TargetSourceIsSomeType: false,
		}

		if targetFieldIsPtr {
			elem := targetField.Type.Elem()
			field.TargetSourceIsSomeType = elem.Kind() == sourceField.Type.Kind() && elem.String() == sourceField.Type.String()
		} else {
			field.TargetSourceIsSomeType = targetField.Type.Kind() == sourceField.Type.Kind() && targetField.Type.String() == sourceField.Type.String()
		}
		context.Fields = append(context.Fields, field)
	}

	parse, err := template.New("gen").Funcs(template.FuncMap{
		"GenZeroCheck":    generateZeroCheckExpr,
		"GenNotZeroCheck": generateNonZeroCheckExpr,
		"ToSnakeCase":     strcase.ToSnake,
	}).Parse(genBindFuncTemplate)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	err = parse.Execute(&builder, context)
	if err != nil {
		return "", err
	}
	return builder.String(), nil
}

type bindContext struct {
	SourcePkgName string
	TargetPkgName string

	ActionName string
	SourceName string
	TargetName string
	FuncName   string

	Fields []fieldContext
}

type fieldContext struct {
	FieldKeyPath string

	TargetField reflect.StructField
	SourceField reflect.StructField

	SetterFuncName       string
	SettNillableFuncName string
	ClearOnNilFuncName   string

	CanSettNillable bool
	CanClearOnNil   bool

	TargetFieldIsPtr       bool
	TargetSourceIsSomeType bool
}

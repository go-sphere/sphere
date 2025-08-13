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

type GenFuncConf struct {
	source        any      // ent entity, e.g. ent.Example
	target        any      // protobuf entity, e.g. entpb.Example
	action        any      // ent operation, e.g. ent.ExampleCreate, ent.ExampleUpdateOne
	IgnoreFields  []string // fields to ignore, e.g.  example.FieldID, example.FieldCreatedAt
	SourcePkgName string   // package name of source, e.g. "ent"
	TargetPkgName string   // package name of target, e.g. "entpb"
}

func NewGenFuncConf(source, target, action any) *GenFuncConf {
	return &GenFuncConf{
		source:        source,
		target:        target,
		action:        action,
		IgnoreFields:  nil,
		SourcePkgName: packageName(source),
		TargetPkgName: packageName(target),
	}
}

func (c *GenFuncConf) WithSourcePkgName(pkgName string) *GenFuncConf {
	c.SourcePkgName = pkgName
	return c
}

func (c *GenFuncConf) WithTargetPkgName(pkgName string) *GenFuncConf {
	c.TargetPkgName = pkgName
	return c
}

func (c *GenFuncConf) WithIgnoreFields(fields ...string) *GenFuncConf {
	c.IgnoreFields = fields
	return c
}

func GenBindFunc(conf *GenFuncConf) (string, error) {
	actionName := getStructName(conf.action)
	sourceName := getStructName(conf.source)
	targetName := getStructName(conf.target)
	funcName := strings.Replace(actionName, sourceName, "", 1) + sourceName

	keys, sourceFields := getPublicFields(conf.source, strcase.ToSnake)
	_, targetFields := getPublicFields(conf.target, strcase.ToSnake)
	_, actionMethods := getPublicMethods(conf.action, strcase.ToSnake)

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
	table := getStructName(conf.source)

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
		"GenZeroCheck":    genZeroCheck,
		"GenNotZeroCheck": genNotZeroCheck,
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

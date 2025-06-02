package bind

import (
	"fmt"
	"reflect"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"
)

type GenFuncConf struct {
	source        any
	target        any
	action        any
	IgnoreFields  []string
	SourcePkgName string
	TargetPkgName string
}

func NewGenFuncConf(source, target, action any) *GenFuncConf {
	return &GenFuncConf{
		source:        source,
		target:        target,
		action:        action,
		IgnoreFields:  nil,
		SourcePkgName: "ent",
		TargetPkgName: "entpb",
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

func GenBindFunc(conf *GenFuncConf) string {
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

	for _, n := range keys {
		if ignoreFields[n] {
			continue
		}
		targetField, ok := targetFields[n]
		if !ok {
			continue
		}
		sourceField, ok := sourceFields[n]
		if !ok {
			continue
		}

		setter, hasSetter := actionMethods[strcase.ToSnake(fmt.Sprintf("Set%s", targetField.Name))]
		if !hasSetter {
			continue
		}
		settNillable, hasSettNillable := actionMethods[strcase.ToSnake(fmt.Sprintf("SetNillable%s", targetField.Name))]
		clearOnNil, hasClearOnNil := actionMethods[strcase.ToSnake(fmt.Sprintf("Clear%s", targetField.Name))]
		targetFieldIsPtr := targetField.Type.Kind() == reflect.Ptr

		field := fieldContext{
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
		"GenZeroCheck": genZeroCheck,
		"ToSnakeCase":  strcase.ToSnake,
	}).Parse(genBindFuncTemplate)
	if err != nil {
		return ""
	}
	var builder strings.Builder
	err = parse.Execute(&builder, context)
	if err != nil {
		return ""
	}
	return builder.String()
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

const genBindFuncTemplate = `
func {{.FuncName}}(source *{{.SourcePkgName}}.{{.ActionName}}, target *{{.TargetPkgName}}.{{.TargetName}}, options ...bind.Option) *{{.SourcePkgName}}.{{.ActionName}} {
	option := bind.NewBindOptions(options...)
{{- range .Fields}}
	if option.CanSetField("{{ToSnakeCase .SourceField.Name}}") {
		{{- if .TargetFieldIsPtr}} {{/* 当目标字段是指针类型 */}}
			{{- if .CanSettNillable}} {{/* 如果存在SetNillable方法，直接使用 */}}
				{{- if .CanClearOnNil}} {{/* 如果存在ClearOnNil，判断是否需要使用 */}}
					if target.{{.TargetField.Name}} == nil && option.ClearOnNil("{{ToSnakeCase .SourceField.Name}}") {
						source.{{.ClearOnNilFuncName}}()
					} else {
						source.{{.SettNillableFuncName}}(target.{{.TargetField.Name}})
					}
				{{- else}}
					source.{{.SettNillableFuncName}}(target.{{.TargetField.Name}})
				{{- end}}
			{{- else}} {{/* 否则使用普通Setter方法，但需要解引用 */}}
				if target.{{.TargetField.Name}} != nil {
        			{{- if .TargetSourceIsSomeType}} {{/* 如果源和目标是相同类型，直接赋值 */}}
						source.{{.SetterFuncName}}(*target.{{.TargetField.Name}}) 
        			{{- else}} {{/* 如果类型不同，需要进行类型转换 */}}
						source.{{.SetterFuncName}}({{.SourceField.Type.String}}(*target.{{.TargetField.Name}}))
        			{{- end}}
				}
			{{- end}}
		{{- else -}} {{/* 当目标字段不是指针类型 */}}
			if !option.IgnoreSetZero("{{ToSnakeCase .SourceField.Name}}") || !({{GenZeroCheck "target" .TargetField}}) {
        		{{- if .TargetSourceIsSomeType}} {{/* 如果源和目标是相同类型，直接赋值 */}}
					source.{{.SetterFuncName}}(target.{{.TargetField.Name}}) 
        		{{- else}} {{/* 如果类型不同，需要进行类型转换 */}}
					source.{{.SetterFuncName}}({{.SourceField.Type.String}}(target.{{.TargetField.Name}}))
        		{{- end}}
    		}
		{{- end}}
	}
{{- end}}
	return source
}
`

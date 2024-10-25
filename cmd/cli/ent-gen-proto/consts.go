package main

import (
	"entgo.io/ent/entc/gen"
	"entgo.io/ent/schema/field"
	"regexp"
)

var (
	snake  = gen.Funcs["snake"].(func(string) string)
	pascal = gen.Funcs["pascal"].(func(string) string)
	camel  = gen.Funcs["camel"].(func(string) string)
)

const protoTpl = `
syntax = "proto3";

package {{.Package}};
{{range .Imports -}}
import "{{.}}";
{{- end -}}

{{range .Schemas}}
	{{- range .Fields -}}
		{{- if .Enum}}
enum {{.Enum.Name}} {
			{{- range .Enum.Values}}
	{{.Name}} = {{.Index}}; // {{.Origin}}
			{{- end}}
}
		{{- end}}
	{{- end}}
{{- end}}
{{range .Schemas}}
message {{.Name}} {
	{{- range .Fields}}
	{{if .Optional}}optional {{end}}{{.ProtoType}} {{.Name}} = {{.Index}};{{if .Comment}} // {{.Comment}}{{end}}
	{{- end}}
}
{{end}}
`

var (
	protoTypeMap = map[field.Type]string{
		field.TypeInvalid: "google.protobuf.Any",
		field.TypeBool:    "bool",
		field.TypeTime:    "google.protobuf.Timestamp",
		field.TypeJSON:    "google.protobuf.Any",
		field.TypeUUID:    "string",
		field.TypeBytes:   "bytes",
		field.TypeEnum:    "string",
		field.TypeString:  "string",
		field.TypeOther:   "string",
		field.TypeInt8:    "int32",
		field.TypeInt16:   "int32",
		field.TypeInt32:   "int32",
		field.TypeInt:     "int32",
		field.TypeInt64:   "int64",
		field.TypeUint8:   "int32",
		field.TypeUint16:  "int32",
		field.TypeUint32:  "int32",
		field.TypeUint:    "int32",
		field.TypeUint64:  "int64",
		field.TypeFloat32: "float",
		field.TypeFloat64: "double",
	}
	goType2ProtoBuildInTypes = map[string]string{
		"int":     "int32",
		"int8":    "int32",
		"int16":   "int32",
		"int32":   "int32",
		"int64":   "int64",
		"uint":    "int32",
		"uint8":   "int32",
		"uint16":  "int32",
		"uint32":  "int32",
		"uint64":  "int64",
		"float32": "float",
		"float64": "double",
		"bool":    "bool",
		"string":  "string",
		"bytes":   "bytes",
	}
)

var (
	goTypeArrayRegexp = regexp.MustCompile(`^\[\](.*)$`)
	goTypeMapRegexp   = regexp.MustCompile(`^map\[(.*)\](.*)$`)
)

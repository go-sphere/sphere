package parser

import (
	"github.com/TBXark/sphere/cmd/protoc-gen-sphere/generate/log"
	bindingpb "github.com/TBXark/sphere/proto/binding/sphere/binding"
	"google.golang.org/protobuf/compiler/protogen"
)

type QueryFormField struct {
	Name  string
	Field *protogen.Field
}

func GinQueryForm(m *protogen.Method, method string, pathVars []URIParamsField) []QueryFormField {
	var fields []QueryFormField
	params := make(map[string]struct{}, len(pathVars))
	for _, v := range pathVars {
		params[v.Name] = struct{}{}
	}
	for _, field := range m.Input.Fields {
		name := string(field.Desc.Name())
		if _, ok := params[name]; ok {
			continue
		}
		if checkBindingLocation(m.Input, field, bindingpb.BindingLocation_BINDING_LOCATION_QUERY) {
			fields = append(fields, QueryFormField{
				Name:  name,
				Field: field,
			})
		} else {
			if _, ok := NoBodyMethods[method]; ok {
				log.Error("Method `%s.%s` parameter `%s` is not bound to either query or uri. File: `%s`, Field: `%s`",
					m.Parent.Desc.Name(),
					m.Desc.Name(),
					name,
					m.Parent.Location.SourceFile,
					m.Input.Desc.Name(),
				)
			}
		}
	}
	return fields
}

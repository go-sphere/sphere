package parser

import (
	"net/http"

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
			formName := parseFieldSphereTag(field, "form", name)
			if formName != "" {
				fields = append(fields, QueryFormField{
					Name:  formName,
					Field: field,
				})
			} else if method == http.MethodGet || method == http.MethodDelete {
				log.Warn("%s `%s`: field `%s` is not bound to query, but it is used in method `%s`", m.Parent.Location.SourceFile, m.Parent.Desc.Name(), name, m.Desc.Name())
			}
		}
	}
	return fields
}

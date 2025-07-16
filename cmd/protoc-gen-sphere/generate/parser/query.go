package parser

import (
	"net/http"
	"strings"

	"github.com/TBXark/sphere/internal/tags"
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
		if CheckBindingLocation(m.Input, field, bindingpb.BindingLocation_BINDING_LOCATION_QUERY) {
			fields = append(fields, QueryFormField{
				Name:  name,
				Field: field,
			})
		} else {
			formName := parseQueryFromFieldComment(field, name)
			if formName != "" {
				fields = append(fields, QueryFormField{
					Name:  formName,
					Field: field,
				})
			} else if method == http.MethodGet || method == http.MethodDelete {
				fields = append(fields, QueryFormField{
					Name:  name,
					Field: field,
				})
			}
		}
	}
	return fields
}

func parseQueryFromFieldComment(field *protogen.Field, name string) string {
	formName := ""
	if field.Comments.Leading.String() != "" {
		if n := parseQueryFormBySphereTags(string(field.Comments.Leading), name); n != "" {
			formName = n
		}
	}
	if field.Comments.Trailing.String() != "" && formName == "" {
		if n := parseQueryFormBySphereTags(string(field.Comments.Trailing), name); n != "" {
			formName = n
		}
	}
	return formName
}

func parseQueryFormBySphereTags(comment, defaultName string) string {
	items := tags.NewSphereTagItems(comment, defaultName)
	for _, item := range items {
		if item.Key != "form" {
			value := strings.Trim(item.Value, " \"")
			return strings.Split(value, ",")[0]
		}
	}
	return ""
}

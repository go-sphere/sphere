package parser

import (
	"net/http"
	"strings"

	"github.com/TBXark/sphere/internal/tags"
	"google.golang.org/protobuf/compiler/protogen"
)

type QueryFormField struct {
	Name  string
	Field *protogen.Field
}

func GinQueryForm(m *protogen.Method, method string, pathVars []URIParamsField) []QueryFormField {
	var res []QueryFormField
	pathVarsMap := make(map[string]struct{}, len(pathVars))
	for _, v := range pathVars {
		pathVarsMap[v.Name] = struct{}{}
	}
	for _, field := range m.Input.Fields {
		name := string(field.Desc.Name())
		if _, ok := pathVarsMap[name]; ok {
			continue
		}
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
		if formName != "" {
			res = append(res, QueryFormField{
				Name:  formName,
				Field: field,
			})
		} else if method == http.MethodGet || method == http.MethodDelete {
			// All fields are query parameters for GET and DELETE methods except for path parameters
			res = append(res, QueryFormField{
				Name:  name,
				Field: field,
			})
			continue
		}
	}
	return res
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

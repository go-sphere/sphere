package parser

import (
	"net/http"
	"strings"

	"github.com/TBXark/sphere/contrib/sphere-shared/tags"
	"google.golang.org/protobuf/compiler/protogen"
)

func QueryValue(m *protogen.Method, method string, pathVars []string) []string {
	var res []string
	pathVarsMap := make(map[string]struct{}, len(pathVars))
	for _, v := range pathVars {
		pathVarsMap[v] = struct{}{}
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
			res = append(res, formName)
		} else if method == http.MethodGet || method == http.MethodDelete {
			// All fields are query parameters for GET and DELETE methods except for path parameters
			res = append(res, name)
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

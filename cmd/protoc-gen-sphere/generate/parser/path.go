package parser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/TBXark/sphere/cmd/protoc-gen-sphere/generate/log"
	bindingpb "github.com/TBXark/sphere/proto/binding/sphere/binding"
	"google.golang.org/protobuf/compiler/protogen"
)

func GinRoute(protoPath string) (string, error) {
	if protoPath == "" {
		return "", fmt.Errorf("proto path cannot be empty")
	}
	result := protoPath
	// 1.  {param=literal/*} or {param=literal/**}
	complexLiteralRegex := regexp.MustCompile(`\{([^}=]+)=([^}*]+)/(\*+)\}`)
	result = complexLiteralRegex.ReplaceAllStringFunc(result, func(match string) string {
		matches := complexLiteralRegex.FindStringSubmatch(match)
		if len(matches) >= 4 {
			paramName := cleanParamName(matches[1])
			literalPart := matches[2]
			wildcardPart := matches[3]

			if wildcardPart == "**" {
				// {path=assets/**} -> /assets/*path
				return "/" + literalPart + "/*" + paramName
			} else {
				// {path=assets/*} -> /assets/:path
				return "/" + literalPart + "/:" + paramName
			}
		}
		return match
	})
	// 2. {param=literal} -> /literal
	literalRegex := regexp.MustCompile(`\{([^}=]+)=([^}*/]+)\}`)
	result = literalRegex.ReplaceAllStringFunc(result, func(match string) string {
		matches := literalRegex.FindStringSubmatch(match)
		if len(matches) >= 3 {
			return "/" + matches[2]
		}
		return match
	})
	// 3. {param=**} -> /*param
	doubleWildcardRegex := regexp.MustCompile(`\{([^}=]+)=\*\*\}`)
	result = doubleWildcardRegex.ReplaceAllStringFunc(result, func(match string) string {
		matches := doubleWildcardRegex.FindStringSubmatch(match)
		if len(matches) >= 2 {
			paramName := cleanParamName(matches[1])
			return "/*" + paramName
		}
		return match
	})
	// 4.  {param=*} -> /:param
	singleWildcardRegex := regexp.MustCompile(`\{([^}=]+)=\*\}`)
	result = singleWildcardRegex.ReplaceAllStringFunc(result, func(match string) string {
		matches := singleWildcardRegex.FindStringSubmatch(match)
		if len(matches) >= 2 {
			paramName := cleanParamName(matches[1])
			return "/:" + paramName
		}
		return match
	})
	// 5.  {param} -> /:param
	simpleParamRegex := regexp.MustCompile(`\{([^}=]+)\}`)
	result = simpleParamRegex.ReplaceAllStringFunc(result, func(match string) string {
		matches := simpleParamRegex.FindStringSubmatch(match)
		if len(matches) >= 2 {
			paramName := cleanParamName(matches[1])
			return "/:" + paramName
		}
		return match
	})
	result = regexp.MustCompile(`/+`).ReplaceAllString(result, "/")
	if !strings.HasPrefix(result, "/") {
		result = "/" + result
	}
	if len(result) > 1 && strings.HasSuffix(result, "/") {
		result = strings.TrimSuffix(result, "/")
	}

	return result, nil
}

type URIParamsField struct {
	Name     string
	Wildcard bool
	Field    *protogen.Field
}

func GinURIParams(m *protogen.Method, route string) []URIParamsField {
	var fields []URIParamsField
	params := parseGinRoutePath(route)
	for _, field := range m.Input.Fields {
		name := string(field.Desc.Name())
		wildcard, exist := params[name]
		if exist {
			if checkBindingLocation(m.Input, field, bindingpb.BindingLocation_BINDING_LOCATION_URI) || parseFieldSphereTag(field, "uri", name) != "" {
				fields = append(fields, URIParamsField{
					Name:     name,
					Wildcard: wildcard,
					Field:    field,
				})
			} else {
				log.Warn("%s `%s`: %s field `%s` is not bound to URI, but it is used in route `%s`",
					m.Parent.Location.SourceFile,
					m.Parent.Desc.Name(),
					m.Desc.Name(),
					name,
					route,
				)
			}
		}
	}
	return fields
}

func parseGinRoutePath(route string) map[string]bool {
	params := make(map[string]bool)
	// :param
	namedParamRegex := regexp.MustCompile(`:([a-zA-Z_][a-zA-Z0-9_]*)`)
	namedMatches := namedParamRegex.FindAllStringSubmatch(route, -1)
	for _, match := range namedMatches {
		if len(match) > 1 {
			params[match[1]] = false
		}
	}
	// *param
	wildcardParamRegex := regexp.MustCompile(`\*([a-zA-Z_][a-zA-Z0-9_]*)`)
	wildcardMatches := wildcardParamRegex.FindAllStringSubmatch(route, -1)
	for _, match := range wildcardMatches {
		if len(match) > 1 {
			params[match[1]] = true
		}
	}
	return params
}

func cleanParamName(paramName string) string {
	cleaned := strings.ReplaceAll(paramName, ".", "_")
	reg := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	cleaned = reg.ReplaceAllString(cleaned, "_")
	return cleaned
}

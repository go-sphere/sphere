package parser

import (
	"fmt"
	"regexp"
	"strings"
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

func GinRouteParams(route string) []string {
	var params []string
	// :param
	namedParamRegex := regexp.MustCompile(`:([a-zA-Z_][a-zA-Z0-9_]*)`)
	namedMatches := namedParamRegex.FindAllStringSubmatch(route, -1)
	for _, match := range namedMatches {
		if len(match) > 1 {
			params = append(params, match[1])
		}
	}
	// *param
	wildcardParamRegex := regexp.MustCompile(`\*([a-zA-Z_][a-zA-Z0-9_]*)`)
	wildcardMatches := wildcardParamRegex.FindAllStringSubmatch(route, -1)
	for _, match := range wildcardMatches {
		if len(match) > 1 {
			params = append(params, match[1])
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

package httpz

import (
	"path"

	"github.com/go-sphere/httpx"
)

func lastChar(s string) byte {
	if len(s) == 0 {
		return 0
	}
	return s[len(s)-1]
}

func joinPaths(absolutePath, relativePath string) string {
	if relativePath == "" {
		return absolutePath
	}
	finalPath := path.Join(absolutePath, relativePath)
	if lastChar(relativePath) == '/' && lastChar(finalPath) != '/' {
		return finalPath + "/"
	}
	return finalPath
}

func EndpointsToMatches(base string, endpoints ...[][3]string) map[string]map[string]string {
	matches := make(map[string]map[string]string)
	for _, list := range endpoints {
		for _, route := range list {
			key := route[1]
			inner, ok := matches[key]
			if !ok || inner == nil {
				inner = make(map[string]string)
				matches[key] = inner
			}
			inner[joinPaths(base, route[2])] = route[0]
		}
	}
	return matches
}

func MatchOperation(base string, endpoints [][3]string, operations ...string) func(ctx httpx.Context) bool {
	matches := EndpointsToMatches(base, endpoints)
	opts := make(map[string]struct{}, len(operations))
	for _, opt := range operations {
		opts[opt] = struct{}{}
	}
	return func(ctx httpx.Context) bool {
		if method, ok := matches[ctx.Method()]; ok {
			if opt, exist := method[ctx.FullPath()]; exist {
				if _, match := opts[opt]; match {
					return true
				}
			}
		}
		return false
	}
}

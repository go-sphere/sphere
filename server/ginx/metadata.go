package ginx

import (
	"path"

	"github.com/gin-gonic/gin"
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
			if _, ok := matches[route[1]]; !ok {
				matches[route[1]] = make(map[string]string)
			}
			matches[route[1]][joinPaths(base, route[2])] = route[0]
		}
	}
	return matches
}

func MatchOperation(base string, endpoints [][3]string, operations ...string) func(ctx *gin.Context) bool {
	matches := EndpointsToMatches(base, endpoints)
	opts := make(map[string]struct{}, len(operations))
	for _, opt := range operations {
		opts[opt] = struct{}{}
	}
	return func(ctx *gin.Context) bool {
		if method, ok := matches[ctx.Request.Method]; ok {
			if opt, exist := method[ctx.FullPath()]; exist {
				if _, match := opts[opt]; match {
					return true
				}
			}
		}
		return false
	}
}

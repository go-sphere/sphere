package ginx

import "github.com/gin-gonic/gin"

func metadataToMatches(base string, metadata ...[][3]string) map[string]map[string]string {
	matches := make(map[string]map[string]string)
	for _, list := range metadata {
		for _, route := range list {
			if _, ok := matches[route[1]]; !ok {
				matches[route[1]] = make(map[string]string)
			}
			matches[route[1]][JoinPaths(base, route[2])] = route[0]
		}
	}
	return matches
}

func MatchOperation(base string, metadata [][3]string, operations ...string) func(ctx *gin.Context) bool {
	matches := metadataToMatches(base, metadata)
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

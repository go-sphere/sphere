package ginx

import "github.com/gin-gonic/gin"

func RoutesToMatches(base string, routes ...[][3]string) map[string]map[string]string {
	matches := make(map[string]map[string]string)
	for _, list := range routes {
		for _, route := range list {
			if _, ok := matches[route[1]]; !ok {
				matches[route[1]] = make(map[string]string)
			}
			matches[route[1]][JoinPaths(base, route[2])] = route[0]
		}
	}
	return matches
}

func MatchOperation(route gin.IRouter, operation string, routes [][3]string) func(ctx *gin.Context) bool {
	matches := RoutesToMatches(route.Group("").BasePath(), routes)
	return func(ctx *gin.Context) bool {
		if method, ok := matches[ctx.Request.Method]; ok {
			if opt, exist := method[ctx.FullPath()]; exist {
				return opt == operation
			}
		}
		return false
	}
}

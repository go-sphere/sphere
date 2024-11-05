package ginx

import "github.com/gin-gonic/gin"

func SetContextValue(key string, value interface{}) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set(key, value)
	}
}

func MiddlewaresGroup(middlewares ...gin.HandlerFunc) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		for _, m := range middlewares {
			m(ctx)
			if ctx.IsAborted() {
				return
			}
		}
		ctx.Next()
	}
}

// SetOperationMiddleware set the operation name to the context
// operations is a list of [[operation, method, path]]
func SetOperationMiddleware(base string, operations ...[][3]string) gin.HandlerFunc {
	routes := make(map[string]map[string]string)
	for _, list := range operations {
		for _, route := range list {
			if _, ok := routes[route[1]]; !ok {
				routes[route[1]] = make(map[string]string)
			}
			routes[route[1]][JoinPaths(base, route[2])] = route[0]
		}
	}
	return func(ctx *gin.Context) {
		if list, ok := routes[ctx.Request.Method]; ok {
			if op, exist := list[ctx.FullPath()]; exist {
				ctx.Set("operation", op)
			}
		}
		ctx.Next()
	}
}

func OperationRouteGroup(route gin.IRouter, operations [][3]string, middlewares ...gin.HandlerFunc) gin.IRouter {
	r := route.Group("/")
	r.Use(SetOperationMiddleware(r.BasePath(), operations))
	r.Use(middlewares...)
	return r
}

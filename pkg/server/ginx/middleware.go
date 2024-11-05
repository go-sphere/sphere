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

func SetOperationMiddleware(operations ...map[string][]string) gin.HandlerFunc {
	routes := make(map[string]map[string]string)
	for _, list := range operations {
		for operation, route := range list {
			if _, ok := routes[route[0]]; !ok {
				routes[route[0]] = make(map[string]string)
			}
			routes[route[0]][route[1]] = operation
		}
	}
	return func(ctx *gin.Context) {
		if list, ok := routes[ctx.Request.Method]; ok {
			if op, exist := list[ctx.Request.URL.Path]; exist {
				ctx.Set("operation", op)
			}
		}
		ctx.Next()
	}
}

func OperationRouteGroup(route gin.IRouter, group string, operationRouteBuilder func(string) map[string][]string, middlewares ...gin.HandlerFunc) gin.IRouter {
	r := route.Group(group)
	r.Use(SetOperationMiddleware(operationRouteBuilder(r.BasePath())))
	r.Use(middlewares...)
	return r
}

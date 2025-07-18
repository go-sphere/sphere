package ginx

import "github.com/gin-gonic/gin"

type HandlerFunc func(ctx *gin.Context) error

type MiddlewareFunc func(next HandlerFunc) HandlerFunc

func MiddlewareChain(middlewares ...MiddlewareFunc) MiddlewareFunc {
	if len(middlewares) == 0 {
		return func(next HandlerFunc) HandlerFunc {
			return next
		}
	}
	return func(next HandlerFunc) HandlerFunc {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

func AdaptMiddleware(middleware ...MiddlewareFunc) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		mid := MiddlewareChain(middleware...)
		handler := mid(func(c *gin.Context) error {
			c.Next()
			return nil
		})
		err := handler(ctx)
		if err != nil {
			AbortWithJsonError(ctx, err)
		}
	}
}

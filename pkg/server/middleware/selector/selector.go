package selector

import "github.com/gin-gonic/gin"

type Matcher interface {
	Match(ctx *gin.Context) bool
}

type MatchFunc func(ctx *gin.Context) bool

func (m MatchFunc) Match(ctx *gin.Context) bool {
	return m(ctx)
}

func NewContextMatcher[T comparable](key string, value T) Matcher {
	return MatchFunc(func(ctx *gin.Context) bool {
		v, ok := ctx.Get(key)
		if !ok {
			return false
		}
		typedValue, ok := v.(T)
		if !ok {
			return false
		}
		return typedValue == value
	})
}

func NewPathMatcher(path string) Matcher {
	return MatchFunc(func(ctx *gin.Context) bool {
		return ctx.FullPath() == path
	})
}

func NewLogicalNotMatcher(matcher Matcher) Matcher {
	return MatchFunc(func(ctx *gin.Context) bool {
		return !matcher.Match(ctx)
	})
}

func NewSelectorMiddleware(matcher Matcher, middlewares ...gin.HandlerFunc) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if matcher.Match(ctx) {
			for _, m := range middlewares {
				m(ctx)
				if ctx.IsAborted() {
					return
				}
			}
		}
		ctx.Next()
	}
}

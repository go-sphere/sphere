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

func NewLogicalNotMatcher(matcher Matcher) Matcher {
	return MatchFunc(func(ctx *gin.Context) bool {
		return !matcher.Match(ctx)
	})
}

func NewLogicalOrMatcher(matchers ...Matcher) Matcher {
	return MatchFunc(func(ctx *gin.Context) bool {
		for _, m := range matchers {
			if m.Match(ctx) {
				return true
			}
		}
		return false
	})
}

func NewLogicalAndMatcher(matchers ...Matcher) Matcher {
	return MatchFunc(func(ctx *gin.Context) bool {
		for _, m := range matchers {
			if !m.Match(ctx) {
				return false
			}
		}
		return true
	})
}

func NewSelectorMiddleware(matcher Matcher, middlewares ...gin.HandlerFunc) gin.HandlersChain {
	chain := make(gin.HandlersChain, 0, len(middlewares))
	for _, middleware := range middlewares {
		chain = append(chain, func(ctx *gin.Context) {
			if matcher.Match(ctx) {
				middleware(ctx)
			}
			ctx.Next()
		})
	}
	return chain
}

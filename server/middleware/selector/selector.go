package selector

import (
	"github.com/go-sphere/httpx"
)

// Matcher defines the interface for request matching logic.
// Implementations determine whether a given request context matches specific criteria.
// Match runs on the request path, possibly concurrently, and should not write
// the response.
type Matcher interface {
	Match(ctx httpx.Context) bool
}

// MatchFunc is a function type that implements the Matcher interface.
// This allows functions to be used directly as matchers without defining new types.
type MatchFunc func(ctx httpx.Context) bool

// Match implements the Matcher interface for MatchFunc.
func (m MatchFunc) Match(ctx httpx.Context) bool {
	return m(ctx)
}

// NewContextMatcher matches when the httpx context value for key equals value.
// The stored value must have type T.
func NewContextMatcher[T comparable](key string, value T) Matcher {
	return MatchFunc(func(ctx httpx.Context) bool {
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

// NewLogicalNotMatcher creates a matcher that inverts the result of another matcher.
func NewLogicalNotMatcher(matcher Matcher) Matcher {
	return MatchFunc(func(ctx httpx.Context) bool {
		return !matcher.Match(ctx)
	})
}

// NewLogicalOrMatcher returns a matcher that is true if any matcher matches. An empty list is false.
func NewLogicalOrMatcher(matchers ...Matcher) Matcher {
	return MatchFunc(func(ctx httpx.Context) bool {
		for _, m := range matchers {
			if m.Match(ctx) {
				return true
			}
		}
		return false
	})
}

// NewLogicalAndMatcher returns a matcher that is true only if every matcher matches. An empty list is true.
func NewLogicalAndMatcher(matchers ...Matcher) Matcher {
	return MatchFunc(func(ctx httpx.Context) bool {
		for _, m := range matchers {
			if !m.Match(ctx) {
				return false
			}
		}
		return true
	})
}

// NewSelectorMiddleware returns one wrapper per input middleware: each wrapper
// runs the inner middleware only when matcher.Match is true, and otherwise
// continues the chain directly. Each wrapper evaluates matcher independently,
// so matcher runs once per wrapped middleware per request. Pass the result to
// Use with the spread operator:
//
//	router.Use(selector.NewSelectorMiddleware(matcher, authMiddleware, permissionMiddleware)...)
func NewSelectorMiddleware(matcher Matcher, middlewares ...httpx.Middleware) []httpx.Middleware {
	val := make([]httpx.Middleware, 0, len(middlewares))
	for _, m := range middlewares {
		val = append(val, func(next httpx.Handler) httpx.Handler {
			// Composed once at registration, so a request only pays for the
			// match.
			matched := m(next)
			return func(ctx httpx.Context) error {
				if matcher.Match(ctx) {
					return matched(ctx)
				}
				return next(ctx)
			}
		})
	}
	return val
}

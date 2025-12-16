package selector

import (
	"github.com/go-sphere/httpx"
)

// Matcher defines the interface for request matching logic.
// Implementations determine whether a given request context matches specific criteria.
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

// NewContextMatcher creates a matcher that checks for a specific value in the Gin context.
// It performs type-safe comparison of context values.
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

// NewLogicalOrMatcher creates a matcher that returns true if any of the provided matchers match.
// It implements logical OR operation across multiple matchers.
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

// NewLogicalAndMatcher creates a matcher that returns true only if all provided matchers match.
// It implements logical AND operation across multiple matchers.
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

// NewSelectorMiddleware creates a chain of middleware that only execute when the matcher condition is met.
// This allows conditional application of middleware based on request characteristics.
func NewSelectorMiddleware(matcher Matcher, middlewares ...httpx.Middleware) httpx.Middleware {
	return func(ctx httpx.Context) error {
		if matcher.Match(ctx) {
			for _, mw := range middlewares {
				if err := mw(ctx); err != nil {
					return err
				}
			}
		}
		return ctx.Next()
	}
}

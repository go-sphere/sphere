// Package selector applies httpx middleware only to requests a [Matcher]
// selects; other requests skip it and continue the chain directly.
//
// The main use is running auth only on the private operations of a generated
// route table: build the predicate with server/httpz.MatchOperation, adapt it
// with [MatchFunc], and wrap the middleware with [NewSelectorMiddleware].
//
// # Usage
//
//	import (
//		"github.com/go-sphere/sphere/server/httpz"
//		"github.com/go-sphere/sphere/server/middleware/selector"
//	)
//
//	private := selector.MatchFunc(httpz.MatchOperation(
//		api.BasePath(), userv1.EndpointsUserService[:], // generated [...][3]string table
//		userv1.OperationUserServiceGetProfile,
//	))
//	api.Use(selector.NewSelectorMiddleware(private, authMiddleware)...) // e.g. from middleware/auth
//
// [NewSelectorMiddleware] returns one wrapper per input middleware, so spread
// the result into Use. [NewLogicalAndMatcher] with no matchers is true;
// [NewLogicalOrMatcher] with no matchers is false.
//
// Composition warning: httpz.MatchOperation fails closed — it reports true
// when the route pattern is indeterminate. Wrapping it in
// [NewLogicalNotMatcher] inverts that into "skip the middleware", which is the
// unsafe direction. To express "everything except X", select X positively
// and attach the middleware to the outer group instead of using Not.
package selector

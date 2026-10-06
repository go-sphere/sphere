// Package httpz is the response-convention layer on top of
// github.com/go-sphere/httpx. Handlers return (T, error); the wrappers recover
// panics, render errors as an [ErrorResponse], and wrap successful values in a
// [DataResponse].
//
// Use [WithJson] for JSON endpoints, [WithText] for plain-text endpoints,
// [WithFormFileReader] or [WithFormFileBytes] for multipart uploads, and
// [WithSSE] for server-streaming endpoints. Install [AbortWithJsonError] as the
// engine's error handler so errors returned by middleware use the same
// envelope as errors returned by handlers.
//
// # Usage
//
//	import (
//		"github.com/go-sphere/httpx"
//		"github.com/go-sphere/httpx/stdx"
//		"github.com/go-sphere/sphere/server/httpz"
//	)
//
//	engine := stdx.New(
//		stdx.WithAddr(":8080"),
//		stdx.WithErrorHandler(httpz.AbortWithJsonError),
//	)
//	api := engine.Group("/api")
//	api.GET("/users/:id", httpz.WithJson(func(ctx httpx.Context) (User, error) {
//		user, ok := users[ctx.Param("id")]
//		if !ok {
//			return User{}, httpx.NewNotFoundError("user not found")
//		}
//		return user, nil
//	}))
//	// engine.Start() blocks until engine.Stop(ctx) is called; core/boot
//	// usually owns that lifecycle.
//
// A successful call writes {"success":true,"data":...} with status 200, or
// with a status the handler set through ctx.Status. A failed call writes
// {"success":false,"code":0,"message":"user not found"} with status 404.
//
// # Errors
//
// [AbortWithJsonError] maps errors with the parser installed by
// [SetDefaultErrorParser]; the default is [ParseError], which adds HTTP
// statuses for the storage/storageerr sentinels. ErrorResponse.Code is 0
// unless the error carries an application code. ErrorResponse.Message is the
// error's user-facing message, or the generic status text when it has none.
// The raw err.Error() is exposed only after SetDebugMode(true).
//
// # Other helpers
//
//   - [MatchOperation] and [EndpointsToMatches] turn generated route tables
//     into a predicate for server/middleware/selector.
//   - [MountStdAll] mounts a net/http handler on an httpx router.
//   - [Fs] picks a local directory or an embedded file system.
//   - [StopServer] gracefully stops a [net/http.Server] and force-closes it
//     when the caller's deadline expires.
//   - [Value] reads a typed value from the httpx context store.
package httpz

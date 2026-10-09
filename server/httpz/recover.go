package httpz

import (
	"net/http"
	"runtime/debug"

	"github.com/go-sphere/httpx"
)

// HandlePanic is the shared tail of every recover site: call it from a
// deferred function with the non-nil value recover returned.
//
// http.ErrAbortHandler is re-panicked untouched, so net/http drops the
// connection instead of logging a stack and writing a 500. For any other value
// report is called with the panic value and, when withStack is true, the
// current goroutine's stack (otherwise ""), and then render is called to finish
// the request. render is skipped when the response is already committed:
// status and part of the body are on the wire and a second write would only
// append to them. report may be nil.
//
// HandlePanic must be called on the goroutine that panicked, from the deferred
// function itself, so the stack still contains the panic site.
func HandlePanic(
	ctx httpx.Context,
	rec any,
	withStack bool,
	report func(rec any, stack string),
	render func(ctx httpx.Context),
) {
	// http.ErrAbortHandler is net/http's documented way to abandon a request
	// (usually a client disconnect).
	if rec == http.ErrAbortHandler {
		panic(rec)
	}
	if report != nil {
		stack := ""
		if withStack {
			stack = string(debug.Stack())
		}
		report(rec, stack)
	}
	if ctx.Committed() {
		return
	}
	render(ctx)
}

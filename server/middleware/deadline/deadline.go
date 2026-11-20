// Package deadline provides middleware that bounds how long the handlers
// behind it may take, by putting a deadline on the request context.
//
// The bound is cooperative. Nothing interrupts a handler: the deadline reaches
// whatever the handler passes its context to (database calls, outbound HTTP,
// cache reads), and those calls fail with context.DeadlineExceeded once it
// passes. A handler that ignores its context runs to completion and its
// response is sent as usual.
//
// When the chain returns an error after the deadline has passed, the error is
// wrapped so errors.Is(err, context.DeadlineExceeded) holds, which
// httpz.ParseError maps to 504. An explicit httpx.StatusError in the chain
// still decides the status, and a response that is already committed is not
// changed.
//
// Register it on the groups or routes that need a bound, inside the
// engine-level stack (server/middleware/stack). Leave it off routes that
// stream (SSE) or take large uploads, or give them their own, longer value:
// the deadline covers the whole handler, including the time spent reading the
// request body and writing the response.
package deadline

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-sphere/httpx"
)

// New returns middleware that gives the request context a deadline d from the
// moment the request reaches it. A context that already carries an earlier
// deadline keeps it. The context stays in place after the chain returns, so
// outer middleware still reads the values inner layers attached to it; it is
// cancelled by then. A non-positive d returns middleware that does nothing.
func New(d time.Duration) httpx.Middleware {
	return func(next httpx.Handler) httpx.Handler {
		if d <= 0 {
			return next
		}
		return func(ctx httpx.Context) error {
			c, cancel := context.WithTimeout(ctx.Context(), d)
			defer cancel()
			ctx.SetContext(c)
			err := next(ctx)
			if err != nil && errors.Is(c.Err(), context.DeadlineExceeded) && !errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("%w: %w", context.DeadlineExceeded, err)
			}
			return err
		}
	}
}

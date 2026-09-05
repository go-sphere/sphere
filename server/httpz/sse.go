package httpz

import (
	"context"
	"errors"
	"runtime/debug"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/log"
)

// SSE event types used by WithSSE beyond the default (unnamed) message
// events. Clients must understand both: a stream always terminates with
// either a "done" or an "error" event; a missing terminator means the
// connection was interrupted.
const (
	// SSEEventDone terminates a successful stream. Its data is an empty
	// JSON object.
	SSEEventDone = "done"
	// SSEEventError terminates a failed stream. Its data is the standard
	// ErrorResponse envelope, rendered exactly like AbortWithJsonError
	// renders unary errors (parser, code normalization, debug mode).
	SSEEventError = "error"
)

// defaultSSEHeartbeat is the default comment-frame interval that keeps idle
// streams alive through proxies and load balancers.
const defaultSSEHeartbeat = 15 * time.Second

// errSSEStreamEnded is the cancel cause installed when the response-side
// pump stops (stream complete or client gone); sends observe it as their
// termination reason.
var errSSEStreamEnded = errors.New("httpz: SSE stream ended")

// SSEStream is the second phase of a WithSSE handler: a push-style producer
// that emits messages through send. It runs on its own goroutine and must
// only use values captured in phase one (the standard context, the bound
// request) — never the httpx.Context.
//
// send blocks until the message is written to the client (natural
// backpressure) and returns a non-nil error when the stream is dead (client
// disconnected, request canceled); the producer should stop promptly and
// return. Returning nil terminates the stream with a "done" event; returning
// an error terminates it with an "error" event (or, when nothing was sent
// yet, with a plain JSON error response).
type SSEStream[T any] func(send func(T) error) error

type sseOptions struct {
	heartbeat time.Duration
	retry     time.Duration
}

// SSEOption configures WithSSE.
type SSEOption func(*sseOptions)

// WithSSEHeartbeat sets the interval between keep-alive comment frames on an
// idle stream. Zero or negative disables heartbeats. The default is 15s.
func WithSSEHeartbeat(d time.Duration) SSEOption {
	return func(o *sseOptions) {
		o.heartbeat = d
	}
}

// WithSSERetry advertises a client reconnection delay by emitting a "retry:"
// frame when the stream commits. Zero (the default) emits nothing.
func WithSSERetry(d time.Duration) SSEOption {
	return func(o *sseOptions) {
		o.retry = d
	}
}

// WithSSE wraps a two-phase server-streaming handler as an httpx handler
// producing a Server-Sent Events response.
//
// Phase one (prepare) runs on the handler goroutine with the httpx.Context:
// bind and validate the request there. A prepare error is rendered by
// AbortWithJsonError as a regular JSON error status — nothing has been
// committed yet.
//
// Phase two (the returned SSEStream) runs on its own goroutine. The response
// is committed lazily on the first send: an SSEStream error before any send
// still produces a plain JSON error status, while a later error is delivered
// in-stream as a terminal "error" event (the HTTP status is already 200 by
// then). A successful stream — including one that never sends — terminates
// with a "done" event. Messages are encoded with the same JSON encoding as
// WithJson and delivered as unnamed (default "message" type) events.
//
// Operational notes: the wrapper emits heartbeat comments (see
// WithSSEHeartbeat) so idle streams survive proxy idle timeouts, and the
// response sets X-Accel-Buffering: no for nginx-style proxies; other
// buffering middleware (e.g. gzip) must be skipped for these routes. The
// server's write timeout must exceed the stream lifetime. Graceful shutdown
// does not cancel in-flight streams by itself — the stream ends when its
// context is canceled or its connection is force-closed at the shutdown
// deadline, so producers must honor send errors and context cancellation.
func WithSSE[T any](prepare func(ctx httpx.Context) (SSEStream[T], error), opts ...SSEOption) httpx.Handler {
	conf := sseOptions{heartbeat: defaultSSEHeartbeat}
	for _, opt := range opts {
		opt(&conf)
	}
	return WithRecover("WithSSE panic", func(ctx httpx.Context) error {
		if _, ok := httpx.AsStreamer(ctx); !ok {
			// Fail before running prepare: without the capability the
			// endpoint cannot work at all, and a JSON 500 beats a panic.
			return httpx.InternalServerError(httpx.ErrStreamerNotSupported, "streaming is not supported by this server")
		}
		stream, err := prepare(ctx)
		if err != nil {
			return err
		}
		if stream == nil {
			return httpx.NewInternalServerError("WithSSE prepare returned a nil stream")
		}

		reqCtx := ctx.Context()
		prodCtx, cancel := context.WithCancelCause(reqCtx)
		frames := make(chan T)
		result := make(chan error, 1)

		go func() {
			defer close(frames)
			defer func() {
				if r := recover(); r != nil {
					// The producer goroutine has no net/http recovery above
					// it, so even http.ErrAbortHandler must be absorbed here;
					// it still terminates the stream via the error frame.
					log.Error(
						"WithSSE stream panic",
						log.Any("error", r),
						log.String("stack", string(debug.Stack())),
					)
					result <- httpx.InternalServerError(errInternalServerPanic, "internal server error")
				}
			}()
			send := func(msg T) error {
				select {
				case frames <- msg:
					return nil
				case <-prodCtx.Done():
					return context.Cause(prodCtx)
				}
			}
			result <- stream(send)
		}()

		// Gate: hold the response uncommitted until the producer either
		// emits its first message or finishes.
		select {
		case first, ok := <-frames:
			if !ok {
				cancel(errSSEStreamEnded)
				if err := <-result; err != nil {
					return err // pre-frame failure: plain JSON error status
				}
				return serveSSE(ctx, conf, func(w *httpx.SSEWriter) error {
					return w.SendJSON(SSEEventDone, struct{}{})
				})
			}
			return serveSSE(ctx, conf, func(w *httpx.SSEWriter) error {
				return pumpSSE(w, conf, reqCtx, first, frames, result, cancel)
			})
		case <-reqCtx.Done():
			cancel(context.Cause(reqCtx))
			drainSSE(frames, result)
			return httpx.InternalServerError(context.Cause(reqCtx), "request canceled before the stream started")
		}
	})
}

// serveSSE commits the event stream and runs fn, optionally prefixed with a
// retry frame. Once ServerSentEvents has been called the response is
// committed on the sync adapters and owned by the framework on fiber, so
// errors from this point are not returned to the WithRecover error path —
// write failures normally just mean the client went away.
func serveSSE(ctx httpx.Context, conf sseOptions, fn func(w *httpx.SSEWriter) error) error {
	err := httpx.ServerSentEvents(ctx, func(w *httpx.SSEWriter) error {
		if conf.retry > 0 {
			if err := w.Send(&httpx.SSEEvent{Retry: conf.retry}); err != nil {
				return err
			}
		}
		return fn(w)
	})
	if err != nil {
		log.Debug("httpz: SSE stream terminated with error", log.Err(err))
	}
	return nil
}

// pumpSSE is the response-side pump: it writes the first message, then
// forwards producer messages, heartbeats, and the terminal done/error event.
// All response writes happen here, on the single goroutine the adapter runs
// the stream callback on.
func pumpSSE[T any](
	w *httpx.SSEWriter,
	conf sseOptions,
	reqCtx context.Context,
	first T,
	frames <-chan T,
	result <-chan error,
	cancel context.CancelCauseFunc,
) error {
	defer cancel(errSSEStreamEnded)
	abort := func(reason error) error {
		cancel(reason)
		drainSSE(frames, result)
		return reason
	}

	if err := w.SendJSON("", first); err != nil {
		return abort(err)
	}

	var heartbeat <-chan time.Time
	if conf.heartbeat > 0 {
		ticker := time.NewTicker(conf.heartbeat)
		defer ticker.Stop()
		heartbeat = ticker.C
	}

	for {
		select {
		case msg, ok := <-frames:
			if !ok {
				if err := <-result; err != nil {
					_, resp := buildErrorResponse(err)
					return w.SendJSON(SSEEventError, resp)
				}
				return w.SendJSON(SSEEventDone, struct{}{})
			}
			if err := w.SendJSON("", msg); err != nil {
				return abort(err)
			}
		case <-heartbeat:
			if err := w.Comment(""); err != nil {
				return abort(err)
			}
		case <-reqCtx.Done():
			return abort(context.Cause(reqCtx))
		}
	}
}

// drainSSE unblocks a canceled producer and consumes its result so the
// goroutine can exit.
func drainSSE[T any](frames <-chan T, result <-chan error) {
	for range frames { //nolint:revive // draining
	}
	<-result
}

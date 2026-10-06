package logger

import (
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/server/httpz"
)

// Log returns middleware that writes one access log after the downstream chain.
// Successful requests are logged with Info; a chain error is logged with Error
// and returned to the caller. Each entry includes status, method, path, query,
// client IP, user-agent, and latency; the message is the request path. Path and
// query are captured before the chain runs. When the chain returns an error
// before a status was written, the status logged is the one httpz.ErrorStatus
// resolves, which consults a parser installed with httpz.SetDefaultErrorParser.
func Log(lg log.BaseLogger) httpx.Middleware {
	return func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			start := time.Now()
			// Capture path and query before the chain: downstream middleware can
			// rewrite the request URL the same way gin-contrib/zap snapshots them
			// first.
			path := ctx.Path()
			query := ctx.RawQuery()
			err := next(ctx)
			attrs := []log.Attr{
				log.Int("status", responseStatus(ctx, err)),
				log.String("method", ctx.Method()),
				log.String("path", path),
				log.String("query", query),
				log.String("ip", ctx.ClientIP()),
				log.String("user-agent", ctx.Header("User-Agent")),
				log.Duration("latency", time.Since(start)),
			}
			if err != nil {
				lg.Error(path, append(attrs, log.Err(err))...)
				return err
			}
			lg.Info(path, attrs...)
			return nil
		}
	}
}

// responseStatus reports the status the client will see. Where the response is
// already written the recorded status is authoritative; when the chain failed
// before anything was written — which is the normal case in a composed chain,
// where the error is rendered at the route rather than at the failing layer —
// the status httpz.ErrorStatus resolves — the one httpz.AbortWithJsonError
// writes with the installed error parser — is what the client will get.
func responseStatus(ctx httpx.Context, err error) int {
	status := ctx.StatusCode()
	if err == nil || status >= http.StatusBadRequest {
		return status
	}
	return httpz.ErrorStatus(err)
}

// RecoveryLog returns middleware that recovers panics from the downstream chain.
// The panic is logged with Error with the panic value; when stack is true
// the entry also includes a stack trace. The request is finished as HTTP 500
// with no body, and the middleware returns nil, so enclosing middleware sees a
// successful chain. Like httpz.WithRecover, it re-panics http.ErrAbortHandler
// without logging it, so net/http can drop the connection.
func RecoveryLog(lg log.BaseLogger, stack bool) httpx.Middleware {
	return func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				// http.ErrAbortHandler is net/http's signal to abandon the
				// request (usually a client gone mid-response); see
				// httpz.WithRecover.
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				attrs := []log.Attr{log.Any("error", rec)}
				if stack {
					attrs = append(attrs, log.String("stack", string(debug.Stack())))
				}
				lg.Error("[Recovery from panic]", attrs...)
				ctx.Status(http.StatusInternalServerError)
				_ = ctx.NoContent(http.StatusInternalServerError)
			}()
			return next(ctx)
		}
	}
}

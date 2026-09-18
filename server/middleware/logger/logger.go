// Package logger is httpx request-access and panic-recovery middleware over log.BaseLogger.
//
// Log records one entry after the downstream chain: Info when Next succeeds,
// Error when Next returns an error. RecoveryLog recovers a panic so the
// process stays up, logs it at Error, and finishes the request as HTTP 500.
//
// RecoveryLog deliberately returns nil after recovering (returning an error
// would let an enclosing middleware write a second response onto the committed
// one), so it must be the innermost recovery layer: composed as
// Log(RecoveryLog(lg, true)) the panic is still logged at Error, with its
// stack, by RecoveryLog — but the outer Log sees a successful chain and records
// the request at Info with status=500 and the request fields. Do not expect
// Log's level alone to reveal a recovered panic.
package logger

import (
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/log"
)

// Log returns middleware that writes one access log after the downstream chain.
// Successful requests are logged with Info; a chain error is logged with Error
// and returned to the caller. Each entry includes status, method, path, query,
// client IP, user-agent, and latency.
func Log(lg log.BaseLogger) httpx.Middleware {
	return httpx.AsMiddleware(LogInterceptor(lg))
}

// LogInterceptor is Log as an httpx.Interceptor, for routers that compose the
// chain at registration instead of adapting one layer per middleware.
func LogInterceptor(lg log.BaseLogger) httpx.Interceptor {
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
// the status carried by the error is what the client will get.
func responseStatus(ctx httpx.Context, err error) int {
	status := ctx.StatusCode()
	if err == nil || status >= http.StatusBadRequest {
		return status
	}
	if _, errStatus, _ := httpx.ParseError(err); errStatus != 0 {
		return int(errStatus)
	}
	return status
}

// RecoveryLog returns middleware that recovers panics from the downstream chain.
// The panic is logged with Error with the panic value; when stack is true
// the entry also includes a stack trace. The request is finished as HTTP 500.
func RecoveryLog(lg log.BaseLogger, stack bool) httpx.Middleware {
	return httpx.AsMiddleware(RecoveryLogInterceptor(lg, stack))
}

// RecoveryLogInterceptor is RecoveryLog as an httpx.Interceptor.
func RecoveryLogInterceptor(lg log.BaseLogger, stack bool) httpx.Interceptor {
	return func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			defer func() {
				rec := recover()
				if rec == nil {
					return
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

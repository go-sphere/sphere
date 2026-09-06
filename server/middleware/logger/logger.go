// Package logger is httpx request-access and panic-recovery middleware over log.BaseLogger.
//
// Log records one entry after the downstream chain: Info when Next succeeds,
// Error when Next returns an error. RecoveryLog recovers a panic so the
// process stays up, logs it at Error, and finishes the request as HTTP 500.
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
	return func(ctx httpx.Context) error {
		start := time.Now()
		// Capture path and query before Next: downstream middleware can rewrite
		// the request URL the same way gin-contrib/zap snapshots them first.
		path := ctx.Path()
		query := ctx.RawQuery()
		err := ctx.Next()
		attrs := []log.Attr{
			log.Int("status", ctx.StatusCode()),
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

// RecoveryLog returns middleware that recovers panics from the downstream chain.
// The panic is logged with Error with the panic value; when stack is true
// the entry also includes a stack trace. The request is finished as HTTP 500.
func RecoveryLog(lg log.BaseLogger, stack bool) httpx.Middleware {
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
		return ctx.Next()
	}
}

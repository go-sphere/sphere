package telemetry

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/server/httpz"
)

// responseStatus reports the status the client gets, or 0 when it is outside
// 100..599. Where a response is already committed the recorded status is
// authoritative; otherwise an error the chain returned is rendered later by the
// engine, so httpz.ErrorStatus (which honors httpz.SetDefaultErrorParser)
// resolves it.
func responseStatus(ctx httpx.Context, err error) int {
	status := ctx.StatusCode()
	if err != nil && !ctx.Committed() {
		status = httpz.ErrorStatus(err)
	}
	if status < 100 || status > 599 {
		return 0
	}
	return status
}

// clientCanceled reports an error caused by the client going away. httpz maps
// context.Canceled to 500 and swallows returned handler errors via
// AbortWithJsonError, which must not mark the server span or metrics as failed.
func clientCanceled(ctx httpx.Context, reqCtx context.Context, err error) bool {
	if err == nil && ctx != nil {
		err, _ = httpz.Value[error](ctx, "sphere.httpz.aborted_error")
	}
	return err != nil && reqCtx != nil && errors.Is(reqCtx.Err(), context.Canceled) && errors.Is(err, context.Canceled)
}

func isServerError(status int) bool { return status >= http.StatusInternalServerError }

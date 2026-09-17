package httpz

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-sphere/sphere/log"
)

// StopServer is the net/http graceful-stop sequence used by Kratos:
// Shutdown(ctx) waits for in-flight requests; if that wait is cut short by
// the caller's context, leftover connections are force-closed with Close.
//
// In-flight request contexts are not canceled during the graceful window —
// handlers that can finish in time are allowed to. A force-close resolves the
// stop successfully (the degradation is logged), so a nil result does not
// always mean the drain completed gracefully.
func StopServer(ctx context.Context, server *http.Server) error {
	if server == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	err := server.Shutdown(ctx)
	if err == nil {
		return nil
	}
	if ctx.Err() == nil {
		// Shutdown failed for a reason other than the caller's context (for
		// example a listener or connection close error). Force-close anyway so
		// nothing is left serving, but report the original cause.
		if closeErr := server.Close(); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
			return errors.Join(err, closeErr)
		}
		return err
	}
	log.Warn("http server couldn't stop gracefully in time, doing force stop")
	if closeErr := server.Close(); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
		return closeErr
	}
	return nil
}

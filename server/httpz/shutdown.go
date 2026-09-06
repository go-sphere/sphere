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
// handlers that can finish in time are allowed to. Force-close is only the
// timeout fallback, matching transport/http.Server.Stop in Kratos.
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
		return err
	}
	log.Warn("http server couldn't stop gracefully in time, doing force stop")
	if closeErr := server.Close(); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
		return closeErr
	}
	return nil
}

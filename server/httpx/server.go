package httpx

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// ListenAndAutoShutdown starts an HTTP server and automatically handles graceful shutdown.
// It listens for context cancellation to trigger shutdown with the specified timeout.
// Returns any error from server startup or shutdown, prioritizing startup errors.
func ListenAndAutoShutdown(ctx context.Context, server *http.Server, closeTimeout time.Duration) error {
	errChan := make(chan error, 1)
	go func() {
		defer close(errChan)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), closeTimeout)
		defer cancel()
		shutdownErr := server.Shutdown(shutdownCtx)
		listenErr := <-errChan
		if listenErr != nil {
			return listenErr
		}
		return shutdownErr
	case err := <-errChan:
		return err
	}
}

// Start begins serving HTTP requests on the configured address.
// It ignores http.ErrServerClosed which is expected during graceful shutdown.
// Returns any other error that occurs during server startup.
func Start(server *http.Server) error {
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Close gracefully shuts down the HTTP server using the provided context.
// It handles nil server gracefully and returns any shutdown error.
func Close(ctx context.Context, server *http.Server) error {
	if server == nil {
		return nil
	}
	err := server.Shutdown(ctx)
	if err != nil {
		return err
	}
	return nil
}

package ginx

import (
	"context"
	"errors"
	"net/http"
	"time"
)

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

func Start(server *http.Server) error {
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

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

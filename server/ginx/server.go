package ginx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
)

func ListenAndAutoShutdown(ctx context.Context, server *http.Server, closeTimeout time.Duration) error {
	errChan := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("server listen error: %w", err)
		}
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), closeTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Errorw("graceful shutdown failed", logfields.Error(err))
			return fmt.Errorf("server shutdown error: %w", err)
		}
		log.Info("server shutdown gracefully")
		return nil
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

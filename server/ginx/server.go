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
	closeChan := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), closeTimeout)
			defer cancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				log.Errorw("close server error", logfields.Error(err))
				errChan <- err
			}
		case <-closeChan: // 防止goroutine泄露
			return
		}
	}()
	defer close(closeChan)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}
	select {
	case err := <-errChan:
		return err
	default:
		return nil
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

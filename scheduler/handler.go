package scheduler

import (
	"context"
	"fmt"

	"github.com/go-sphere/sphere/core/safe"
)

// RecoverHandler wraps a HandlerFunc and converts panics into errors.
func RecoverHandler(handler HandlerFunc) HandlerFunc {
	return func(ctx context.Context) (err error) {
		defer safe.Recover(func(v any) {
			err = panicError(v)
		})
		return handler(ctx)
	}
}

// RecoverPayloadHandler wraps a PayloadHandlerFunc and converts panics into errors.
func RecoverPayloadHandler(handler PayloadHandlerFunc) PayloadHandlerFunc {
	return func(ctx context.Context, payload []byte) (err error) {
		defer safe.Recover(func(v any) {
			err = panicError(v)
		})
		return handler(ctx, payload)
	}
}

// panicError converts a recovered panic value into an error. An error panic
// value is wrapped with %w so its identity survives for errors.Is/As; anything
// else is rendered as-is.
func panicError(v any) error {
	if err, ok := v.(error); ok {
		return fmt.Errorf("scheduler: handler panic: %w", err)
	}
	return fmt.Errorf("scheduler: handler panic: %v", v)
}

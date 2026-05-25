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
			err = fmt.Errorf("scheduler: handler panic: %v", v)
		})
		return handler(ctx)
	}
}

// RecoverPayloadHandler wraps a PayloadHandlerFunc and converts panics into errors.
func RecoverPayloadHandler(handler PayloadHandlerFunc) PayloadHandlerFunc {
	return func(ctx context.Context, payload []byte) (err error) {
		defer safe.Recover(func(v any) {
			err = fmt.Errorf("scheduler: handler panic: %v", v)
		})
		return handler(ctx, payload)
	}
}

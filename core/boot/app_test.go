package boot

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestApplicationStartPreservesUnexpectedWrappedCancellation(t *testing.T) {
	task := &mockTask{
		identifier: "wrapped-cancellation",
		startFunc: func(context.Context) error {
			return fmt.Errorf("task failed before shutdown: %w", context.Canceled)
		},
	}

	err := NewApplication(task).Start(context.Background())
	if err == nil {
		t.Fatal("expected wrapped cancellation to be returned")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want wrapped context.Canceled", err)
	}
}

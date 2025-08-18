package metadata

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestValues(t *testing.T) {
	parentKey := "parentKey"
	parentValue := "parentValue"
	parentCtx := context.WithValue(context.Background(), parentKey, parentValue) //nolint

	testData := map[string]any{
		"key1": "value1",
		"key2": 123,
	}

	// Test case 1: WithValues and Value
	ctx := WithValues(parentCtx, testData)

	// Check values from the data map
	if val := ctx.Value("key1"); val != "value1" {
		t.Errorf("ctx.Value(\"key1\") = %v, want %v", val, "value1")
	}
	if val := ctx.Value("key2"); val != 123 {
		t.Errorf("ctx.Value(\"key2\") = %v, want %v", val, 123)
	}

	// Check value from the parent context
	if val := ctx.Value(parentKey); val != parentValue {
		t.Errorf("ctx.Value(parentKey) = %v, want %v", val, parentValue)
	}

	// Check non-existent key
	if val := ctx.Value("non-existent"); val != nil {
		t.Errorf("ctx.Value(\"non-existent\") should be nil, but got %v", val)
	}

	// Check non-string key
	if val := ctx.Value(123); val != nil {
		t.Errorf("ctx.Value(123) should be nil, but got %v", val)
	}

	// Test case 2: WithValues with empty data
	ctxEmpty := WithValues(parentCtx, map[string]any{})
	if ctxEmpty != parentCtx {
		t.Errorf("WithValues with empty map should return the parent context")
	}

	// Test case 3: Deadline, Done, Err
	deadline := time.Now().Add(1 * time.Second)
	cancelCtx, cancel := context.WithDeadline(context.Background(), deadline)

	ctxWithDeadline := WithValues(cancelCtx, testData)

	if d, ok := ctxWithDeadline.Deadline(); !ok || !d.Equal(deadline) {
		t.Errorf("Deadline() did not propagate correctly")
	}

	if ctxWithDeadline.Done() == nil {
		t.Errorf("Done() channel should not be nil")
	}

	if err := ctxWithDeadline.Err(); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Err() did not propagate correctly, got %v", err)
	}
	cancel() // cancel the context
	<-ctxWithDeadline.Done()
	if err := ctxWithDeadline.Err(); !errors.Is(err, context.Canceled) {
		t.Errorf("Err() should be context.Canceled after cancel, but got %v", err)
	}
}

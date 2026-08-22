package task

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type execStub struct {
	id  string
	err error
}

func (t execStub) Identifier() string          { return t.id }
func (t execStub) Start(context.Context) error { return t.err }
func (t execStub) Stop(context.Context) error  { return nil }

func TestExecuteWrapsErrorWithName(t *testing.T) {
	want := errors.New("boom")
	err := execute(context.Background(), "db", execStub{id: "db", err: want}, func(ctx context.Context, current Task) error {
		return current.Start(ctx)
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want it to wrap %v", err, want)
	}
	if !strings.Contains(err.Error(), "db") {
		t.Fatalf("error %q should name the task", err)
	}
}

func TestExecuteKeepsProvokedCancelAsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := execute(ctx, "worker", execStub{id: "worker", err: context.Canceled}, func(ctx context.Context, current Task) error {
		return current.Start(ctx)
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if !strings.Contains(err.Error(), "worker") {
		t.Fatalf("error %q should name the task", err)
	}
}

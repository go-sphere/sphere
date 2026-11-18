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

type panicStub struct {
	id  string
	val any
}

func (t panicStub) Identifier() string          { return t.id }
func (t panicStub) Start(context.Context) error { panic(t.val) }
func (t panicStub) Stop(context.Context) error  { return nil }

func TestExecutePanicPreservesUnwrapAndIsTaskPanic(t *testing.T) {
	sentinel := errors.New("boom-err")
	err := execute(context.Background(), "worker", panicStub{id: "worker", val: sentinel}, func(ctx context.Context, current Task) error {
		return current.Start(ctx)
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want errors.Is(err, sentinel)", err)
	}
	if !isTaskPanic(err) {
		t.Fatalf("isTaskPanic(%v) = false, want true", err)
	}

	strPanic := execute(context.Background(), "worker", panicStub{id: "worker", val: "boom-str"}, func(ctx context.Context, current Task) error {
		return current.Start(ctx)
	})
	if !isTaskPanic(strPanic) || !strings.Contains(strPanic.Error(), "worker panic: boom-str") {
		t.Fatalf("strPanic = %v", strPanic)
	}
}

type badIDErrorTask struct {
	err error
}

func (badIDErrorTask) Identifier() string            { panic("bad id") }
func (t badIDErrorTask) Start(context.Context) error { return t.err }
func (badIDErrorTask) Stop(context.Context) error    { return nil }

func TestExecutePreservesErrorWhenIdentifierPanics(t *testing.T) {
	want := errors.New("start failed")
	err := execute(context.Background(), "fallback-name", badIDErrorTask{err: want}, func(ctx context.Context, current Task) error {
		return current.Start(ctx)
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if isTaskPanic(err) {
		t.Fatalf("panicking Identifier replaced real error with taskPanicError: %v", err)
	}
}

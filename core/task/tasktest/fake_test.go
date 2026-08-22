package tasktest_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/tasktest"
)

func TestFakeHonorsLifecycleContract(t *testing.T) {
	for _, mode := range []tasktest.Mode{tasktest.ModeRunLoop, tasktest.ModeServer, tasktest.ModeOneshot} {
		mode := mode
		t.Run(modeName(mode), func(t *testing.T) {
			tasktest.AssertLifecycleContract(t, func() task.Task {
				f := tasktest.NewFake("contract")
				f.Mode = mode
				return f
			})
		})
	}
}

func TestFakeModeServerIgnoresContext(t *testing.T) {
	f := tasktest.NewFake("http")
	f.Mode = tasktest.ModeServer

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- f.Start(ctx) }()

	select {
	case <-f.Started():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Start to enter")
	}

	cancel()
	select {
	case err := <-errCh:
		t.Fatalf("ModeServer must ignore ctx cancel, got %v", err)
	case <-time.After(80 * time.Millisecond):
	}

	if err := f.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start after Stop: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Start to return after Stop")
	}
}

func modeName(m tasktest.Mode) string {
	switch m {
	case tasktest.ModeServer:
		return "server"
	case tasktest.ModeOneshot:
		return "oneshot"
	default:
		return "runloop"
	}
}

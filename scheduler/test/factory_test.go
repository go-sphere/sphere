// Package test is the cross-driver contract suite for scheduler.Cron and
// task.Task lifecycle of the cron and asynq drivers.
//
// Add a new driver by returning it from cronFactory rather than writing a
// parallel suite. Periodic jobs are not coordinated across processes; these
// tests run a single instance.
package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-sphere/sphere/scheduler"
	asynqscheduler "github.com/go-sphere/sphere/scheduler/asynq"
	cronscheduler "github.com/go-sphere/sphere/scheduler/cron"
	"github.com/go-sphere/sphere/test/redistest"
)

type cronRuntime interface {
	Register(name, spec string, handler scheduler.HandlerFunc) error
	Unregister(name string) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Close() error
}

type cronFactory struct {
	name string
	new  func(tb testing.TB) cronRuntime
}

func cronFactories() []cronFactory {
	return []cronFactory{
		{
			name: "cron",
			new: func(tb testing.TB) cronRuntime {
				tb.Helper()
				s, err := cronscheduler.NewScheduler(cronscheduler.Config{Seconds: true})
				if err != nil {
					tb.Fatalf("create cron scheduler: %v", err)
				}
				tb.Cleanup(func() { _ = s.Close() })
				return s
			},
		},
		{
			name: "asynq",
			new: func(tb testing.TB) cronRuntime {
				tb.Helper()
				t, ok := tb.(*testing.T)
				if !ok {
					tb.Fatalf("asynq scheduler factory requires *testing.T")
				}
				client := redistest.NewTestRedisClient(t)
				s, err := asynqscheduler.NewScheduler(asynqscheduler.Config{
					ShutdownTimeout: time.Second,
				}, asynqscheduler.WithClient(client))
				if err != nil {
					tb.Fatalf("create asynq scheduler: %v", err)
				}
				tb.Cleanup(func() { _ = s.Close() })
				return s
			},
		},
	}
}

func startCronRuntime(tb testing.TB, s cronRuntime) context.CancelFunc {
	tb.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Start(ctx)
	}()
	tb.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		_ = s.Stop(stopCtx)
		cancel()
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, context.Canceled) {
				tb.Fatalf("scheduler start returned: %v", err)
			}
		case <-time.After(2 * time.Second):
			tb.Fatalf("scheduler did not stop")
		}
	})
	return cancel
}

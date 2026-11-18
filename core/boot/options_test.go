package boot

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/sphere/log"
)

type syncTrackingBackend struct {
	synced atomic.Bool
}

func (*syncTrackingBackend) Log(context.Context, log.Level, string, ...log.Attr) {}

func (b *syncTrackingBackend) Sync() error {
	b.synced.Store(true)
	return nil
}

func (b *syncTrackingBackend) With(...log.Option) log.Backend {
	return b
}

func TestWithLoggerBackendIsInstalledBeforeBuilder(t *testing.T) {
	originalBackend := log.With().Backend()
	backend := log.NewNopBackend()
	t.Cleanup(func() { log.InitWithBackends(originalBackend) })

	seenByBuilder := false
	err := Run(new(struct{}), func(*struct{}) (*Application, error) {
		seenByBuilder = log.With().Backend() == backend
		return NewApplication(), nil
	}, WithLoggerBackend(backend), WithShutdownSignals())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !seenByBuilder {
		t.Fatal("builder did not see the configured logger backend")
	}
}

func TestWithLoggerBackendSyncsAfterBuildFailure(t *testing.T) {
	originalBackend := log.With().Backend()
	backend := &syncTrackingBackend{}
	t.Cleanup(func() { log.InitWithBackends(originalBackend) })

	buildErr := errors.New("build failed")
	err := Run(new(struct{}), func(*struct{}) (*Application, error) {
		return nil, buildErr
	}, WithLoggerBackend(backend))
	if !errors.Is(err, buildErr) {
		t.Fatalf("Run error = %v, want %v", err, buildErr)
	}
	if !backend.synced.Load() {
		t.Fatal("logger backend was not synced after build failure")
	}
}

func TestRunBuildFailureJoinsCleanupError(t *testing.T) {
	tests := []struct {
		name       string
		beforeFail bool
	}{
		{name: "before build failure", beforeFail: true},
		{name: "builder failure"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildErr := errors.New("build failed")
			cleanupErr := errors.New("cleanup failed")
			builderCalled := false
			runOptions := []Option{
				func(o *options) {
					o.afterBuildFail = append(o.afterBuildFail, func(context.Context) error {
						return cleanupErr
					})
				},
			}
			if tt.beforeFail {
				runOptions = append(runOptions, func(o *options) {
					o.beforeBuild = append(o.beforeBuild, func(context.Context) error {
						return buildErr
					})
				})
			}

			err := Run(new(struct{}), func(*struct{}) (*Application, error) {
				builderCalled = true
				return nil, buildErr
			}, runOptions...)
			if !errors.Is(err, buildErr) || !errors.Is(err, cleanupErr) {
				t.Fatalf("Run error = %v, want joined build and cleanup errors", err)
			}
			if want := !tt.beforeFail; builderCalled != want {
				t.Fatalf("builder called = %v, want %v", builderCalled, want)
			}
		})
	}
}

func TestWithLoggerBackendSyncsAfterBeforeStartFailure(t *testing.T) {
	originalBackend := log.With().Backend()
	backend := &syncTrackingBackend{}
	t.Cleanup(func() { log.InitWithBackends(originalBackend) })

	probeErr := errors.New("readiness probe failed")
	err := Run(new(struct{}), func(*struct{}) (*Application, error) {
		return NewApplication(), nil
	}, WithLoggerBackend(backend), WithShutdownSignals(), AddBeforeStart(func(context.Context) error {
		return probeErr
	}))
	if !errors.Is(err, probeErr) {
		t.Fatalf("Run error = %v, want %v", err, probeErr)
	}
	if !backend.synced.Load() {
		t.Fatal("logger backend was not synced after a before-start failure")
	}
}

func TestRunNilApplicationIsBuildFailure(t *testing.T) {
	cleanupRan := false
	err := Run(new(struct{}), func(*struct{}) (*Application, error) {
		return nil, nil
	}, WithShutdownSignals(), func(o *options) {
		o.afterBuildFail = append(o.afterBuildFail, func(context.Context) error {
			cleanupRan = true
			return nil
		})
	})
	if err == nil || !strings.Contains(err.Error(), "nil application") {
		t.Fatalf("Run error = %v, want nil application build failure", err)
	}
	if !cleanupRan {
		t.Fatal("after-build-failure hooks did not run")
	}
}

func TestNilOptionIsIgnored(t *testing.T) {
	opts := newOptions(nil, WithShutdownTimeout(time.Second), nil, AddBeforeStart(nil), AddBeforeStop(nil), AddAfterStop(nil))
	if opts.shutdownTimeout != time.Second {
		t.Fatalf("shutdownTimeout = %v, want %v", opts.shutdownTimeout, time.Second)
	}
	if len(opts.beforeStart) != 0 || len(opts.beforeStop) != 0 || len(opts.afterStop) != 0 {
		t.Fatalf("nil hooks were not skipped: %+v", opts)
	}
	if err := runHooks(context.Background(), []Hook{nil}, "test"); err != nil {
		t.Fatalf("runHooks with nil hook = %v, want nil", err)
	}
}

func TestWithLoggerBackendSyncsAfterLaterAfterStopHooks(t *testing.T) {
	originalBackend := log.With().Backend()
	backend := &syncTrackingBackend{}
	t.Cleanup(func() { log.InitWithBackends(originalBackend) })

	var syncedDuringAfterStop bool
	err := Run(new(struct{}), func(*struct{}) (*Application, error) {
		return NewApplication(), nil
	}, WithLoggerBackend(backend), WithShutdownSignals(), AddAfterStop(func(context.Context) error {
		syncedDuringAfterStop = backend.synced.Load()
		return nil
	}))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if syncedDuringAfterStop {
		t.Fatal("logger backend synced before user AddAfterStop hook ran")
	}
	if !backend.synced.Load() {
		t.Fatal("logger backend was not synced after shutdown")
	}
}

func TestWithLoggerBackendIgnoresNil(t *testing.T) {
	originalBackend := log.With().Backend()
	sentinel := &syncTrackingBackend{}
	log.InitWithBackends(sentinel)
	t.Cleanup(func() { log.InitWithBackends(originalBackend) })

	var typedNil *syncTrackingBackend
	err := Run(new(struct{}), func(*struct{}) (*Application, error) {
		if log.With().Backend() != sentinel {
			t.Fatal("nil logger backend replaced global backend")
		}
		return NewApplication(), nil
	}, WithLoggerBackend(nil), WithLoggerBackend(typedNil), WithShutdownSignals())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestRunHandlesNilAndPanickingBuilderAndStopPanic(t *testing.T) {
	t.Run("nil builder", func(t *testing.T) {
		err := Run[struct{}](new(struct{}), nil, WithShutdownSignals())
		if err == nil || !strings.Contains(err.Error(), "builder is nil") {
			t.Fatalf("Run error = %v, want builder is nil", err)
		}
	})

	t.Run("panicking builder runs afterBuildFail", func(t *testing.T) {
		originalBackend := log.With().Backend()
		backend := &syncTrackingBackend{}
		t.Cleanup(func() { log.InitWithBackends(originalBackend) })

		panicErr := errors.New("builder boom")
		err := Run(new(struct{}), func(*struct{}) (*Application, error) {
			panic(panicErr)
		}, WithLoggerBackend(backend))
		if !errors.Is(err, panicErr) {
			t.Fatalf("Run error = %v, want %v", err, panicErr)
		}
		if !backend.synced.Load() {
			t.Fatal("logger backend was not synced after builder panic")
		}
	})

	t.Run("panicking Stop still runs afterStop", func(t *testing.T) {
		panicErr := errors.New("stop boom")
		var afterStopRan atomic.Bool
		err := run(context.Background(), &mockTask{
			identifier: "panic-stop",
			startFunc:  func(context.Context) error { return nil },
			stopFunc:   func(context.Context) error { panic(panicErr) },
		}, newOptions(WithShutdownTimeout(time.Second), AddAfterStop(func(context.Context) error {
			afterStopRan.Store(true)
			return nil
		})))
		if !errors.Is(err, panicErr) {
			t.Fatalf("run error = %v, want %v", err, panicErr)
		}
		if !afterStopRan.Load() {
			t.Fatal("afterStop hook did not run after Stop panic")
		}
	})
}

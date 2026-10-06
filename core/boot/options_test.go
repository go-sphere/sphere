package boot

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

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

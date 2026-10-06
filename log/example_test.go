package log_test

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/go-sphere/sphere/log"
)

// printBackend is a deterministic Backend for examples: it prints the level,
// message, preset attrs, and per-call attrs without a timestamp.
type printBackend struct {
	attrs map[string]any
}

func (b printBackend) Log(_ context.Context, level log.Level, msg string, attrs ...log.Attr) {
	line := fmt.Sprintf("level=%d msg=%q", level, msg)
	for _, k := range slices.Sorted(maps.Keys(b.attrs)) {
		line += fmt.Sprintf(" %s=%v", k, b.attrs[k])
	}
	for _, a := range attrs {
		line += fmt.Sprintf(" %s=%v", a.Key, a.Value)
	}
	fmt.Println(line)
}

func (b printBackend) Sync() error { return nil }

func (b printBackend) With(options ...log.Option) log.Backend {
	o := log.NewOptions(options...)
	merged := maps.Clone(b.attrs)
	if merged == nil {
		merged = map[string]any{}
	}
	maps.Copy(merged, o.Attrs)
	return printBackend{attrs: merged}
}

// Install a backend on the global logger, log structured fields, derive a
// child logger, and flush before exit.
func ExampleInitWithBackends() {
	prev := log.With().Backend() // only so the example can restore the global
	defer log.InitWithBackends(prev)

	log.InitWithBackends(printBackend{})

	log.Info("server started", log.String("addr", ":8080"))
	userLog := log.With(log.WithAttrs(map[string]any{"component": "user"}))
	userLog.Errorf("load user %d failed", 42)
	if err := log.Sync(); err != nil {
		fmt.Println("sync:", err)
	}

	// Output:
	// level=1 msg="server started" addr=:8080
	// level=3 msg="load user 42 failed" component=user
}

type requestIDKey struct{}

// Copy request-scoped values from the context into every entry logged through
// a Context method. Explicit attrs win on key collisions.
func ExampleWrapBackendWithContextMapMerge() {
	backend := log.WrapBackendWithContextMapMerge(printBackend{}, func(ctx context.Context) map[string]any {
		id, ok := ctx.Value(requestIDKey{}).(string)
		if !ok {
			return nil
		}
		return map[string]any{"request_id": id}
	})
	logger := log.NewLogger(backend)

	ctx := context.WithValue(context.Background(), requestIDKey{}, "req-1")
	logger.InfoContext(ctx, "handled", log.Int("status", 200))
	logger.Info("no context") // uses context.Background, so nothing is merged

	// Output:
	// level=1 msg="handled" request_id=req-1 status=200
	// level=1 msg="no context"
}

// Fan out to several backends and release any that own a handle.
func ExampleNewMultiBackend() {
	backend := log.NewMultiBackend(printBackend{}, nil, printBackend{})
	defer func() {
		if closer, ok := backend.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}()

	log.NewLogger(backend).Warn("disk almost full", log.Float64("used", 0.93))

	// Output:
	// level=2 msg="disk almost full" used=0.93
	// level=2 msg="disk almost full" used=0.93
}

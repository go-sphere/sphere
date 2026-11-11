package log_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/go-sphere/sphere/log"
)

// slogCapable is a backend with an slog bridge that records the options it was
// asked for.
type slogCapable struct {
	log.Backend
	logger  *slog.Logger
	options int
}

func (s *slogCapable) SlogLogger(options ...log.Option) *slog.Logger {
	s.options += len(options)
	return s.logger
}

func newSlogCapable() *slogCapable {
	return &slogCapable{Backend: log.NewNopBackend(), logger: slog.New(slog.DiscardHandler)}
}

// TestWrappersForwardSlogBridge pins that MultiBackend and the context-merge
// wrapper keep the slog capability of what they wrap, so boot.WithLoggerBackend
// still installs the bridge, and fall back to slog.Default when nothing below
// them has one.
func TestWrappersForwardSlogBridge(t *testing.T) {
	extract := func(context.Context) []log.Attr { return nil }
	for _, tc := range []struct {
		name    string
		wrap    func(log.Backend) log.Backend
		capable bool
	}{
		{"multi first", func(b log.Backend) log.Backend { return log.NewMultiBackend(b, log.NewNopBackend()) }, true},
		{"multi later", func(b log.Backend) log.Backend { return log.NewMultiBackend(log.NewStdioBackend(), b) }, true},
		{"merge", func(b log.Backend) log.Backend { return log.WrapBackendWithContextMerge(b, extract) }, true},
		{"merge over multi", func(b log.Backend) log.Backend {
			return log.WrapBackendWithContextMerge(log.NewMultiBackend(log.NewStdioBackend(), b), extract)
		}, true},
		{"multi without bridge", func(log.Backend) log.Backend {
			return log.NewMultiBackend(log.NewStdioBackend(), log.NewNopBackend())
		}, false},
		{"merge without bridge", func(log.Backend) log.Backend {
			return log.WrapBackendWithContextMerge(log.NewStdioBackend(), extract)
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			child := newSlogCapable()
			sb, ok := tc.wrap(child).(log.SlogBackend)
			if !ok {
				t.Fatal("wrapper does not implement log.SlogBackend")
			}
			got := sb.SlogLogger(log.AddCaller())
			if tc.capable {
				if got != child.logger || child.options != 1 {
					t.Fatalf("logger = %p (options %d), want the child's logger with the option forwarded", got, child.options)
				}
				return
			}
			if got != slog.Default() {
				t.Fatalf("logger = %p, want slog.Default()", got)
			}
		})
	}
}

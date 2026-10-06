// Package log is a backend-agnostic structured logger whose fields are
// [log/slog.Attr] values.
//
// The process-wide logger starts as a zero [StdioBackend] (logfmt lines with
// a UTC timestamp, Debug and above; Error goes to stderr, other levels to
// stdout). Call [InitWithBackends] early in main to replace it. The
// package-level [Debug], [Info], [Warn], and [Error] functions, with their
// Context and f variants, log through that global and are safe for
// concurrent use; the global is swapped atomically. Build fields with
// [String], [Int], [Any], [Err], and the other constructors. [With] derives
// a child [Logger]; [NewLogger] wraps any [Backend] without touching the
// global; [Sync] flushes the global backend.
//
// # Usage
//
//	import (
//		"github.com/go-sphere/sphere/log"
//		"github.com/go-sphere/sphere/log/zapx"
//	)
//
//	backend := zapx.NewBackend(zapx.NewDefaultConfig(), log.WithName("api"))
//	log.InitWithBackends(backend)
//	defer func() {
//		_ = log.Sync()
//		_ = backend.Close()
//	}()
//
//	log.Info("server started", log.String("addr", ":8080"))
//	userLog := log.With(log.WithAttrs(map[string]any{"component": "user"}))
//	userLog.Errorf("load user %d failed", 42)
//
// # Backends
//
// A [Backend] implements Log, Sync, and With. Close is not on the interface:
// type-assert [io.Closer] when a backend owns a handle (StdioBackend has
// nothing to close; zapx.Backend, [MultiBackend], and the
// [WrapBackendWithContextMerge] wrapper forward or implement Close).
// InitWithBackends neither closes nor syncs the previous backend; the caller
// owns every backend it installs. An empty or all-nil list keeps the current
// logger and prints a warning on stderr; pass [NewNopBackend] to discard
// logs on purpose. Several backends are combined with [NewMultiBackend].
//
// StdioBackend honors [WithMinLevel]. zapx does not; set zapx.Config.Level
// instead. [WithStackAt] attaches a stack at that level and above; it is not
// a level filter.
//
// [WrapBackendWithContextMerge] injects attributes extracted from a
// [context.Context] into every entry logged through the Context methods.
// The non-Context and f methods use [context.Background]; the f methods
// format with [fmt.Sprintf] and carry no attributes.
package log

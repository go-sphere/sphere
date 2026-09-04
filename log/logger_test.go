package log

import (
	"context"
	"errors"
	"testing"
)

// entry is one captured Log call, kept whole so a test can assert on the level,
// the message and the context the facade forwarded.
type entry struct {
	ctx   context.Context
	level Level
	msg   string
	attrs []Attr
}

// recordingBackend records everything it is asked to log. withOptions records the
// options passed to With so the derivation path can be asserted too.
type recordingBackend struct {
	entries     []entry
	withOptions int
	syncErr     error
}

func (r *recordingBackend) Log(ctx context.Context, level Level, msg string, attrs ...Attr) {
	r.entries = append(r.entries, entry{ctx: ctx, level: level, msg: msg, attrs: attrs})
}

func (r *recordingBackend) Sync() error { return r.syncErr }

func (r *recordingBackend) With(options ...Option) Backend {
	r.withOptions += len(options)
	return r
}

func (r *recordingBackend) last(tb testing.TB) entry {
	tb.Helper()
	if len(r.entries) == 0 {
		tb.Fatal("no log entry was recorded")
	}
	return r.entries[len(r.entries)-1]
}

// installRecorder installs a recording backend as the global logger and restores
// whatever was there before, so these tests neither depend on nor disturb the
// ordering of the other tests in this package.
func installRecorder(tb testing.TB) *recordingBackend {
	tb.Helper()
	prev := std.Load()
	tb.Cleanup(func() { std.Store(prev) })

	rec := &recordingBackend{}
	InitWithBackends(rec)
	return rec
}

// TestPackageLevelRouting pins which level each package-level function emits at.
//
// The facade is twelve near-identical one-line bodies that differ only in the
// Level constant, which is exactly the shape where a copy-paste slip is
// invisible on review: the call compiles, the message still appears, and it is
// only wrong in the field, where an Error routed at Debug is dropped by any
// backend with a minimum level and never reaches whoever is paged.
func TestPackageLevelRouting(t *testing.T) {
	for _, tc := range []struct {
		name  string
		level Level
		log   func(msg string)
	}{
		{name: "Debug", level: LevelDebug, log: func(m string) { Debug(m) }},
		{name: "Info", level: LevelInfo, log: func(m string) { Info(m) }},
		{name: "Warn", level: LevelWarn, log: func(m string) { Warn(m) }},
		{name: "Error", level: LevelError, log: func(m string) { Error(m) }},

		{name: "DebugContext", level: LevelDebug, log: func(m string) { DebugContext(context.Background(), m) }},
		{name: "InfoContext", level: LevelInfo, log: func(m string) { InfoContext(context.Background(), m) }},
		{name: "WarnContext", level: LevelWarn, log: func(m string) { WarnContext(context.Background(), m) }},
		{name: "ErrorContext", level: LevelError, log: func(m string) { ErrorContext(context.Background(), m) }},

		{name: "Debugf", level: LevelDebug, log: func(m string) { Debugf("%s", m) }},
		{name: "Infof", level: LevelInfo, log: func(m string) { Infof("%s", m) }},
		{name: "Warnf", level: LevelWarn, log: func(m string) { Warnf("%s", m) }},
		{name: "Errorf", level: LevelError, log: func(m string) { Errorf("%s", m) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := installRecorder(t)
			tc.log("routed")

			got := rec.last(t)
			if got.level != tc.level {
				t.Errorf("%s emitted level %d, want %d", tc.name, got.level, tc.level)
			}
			if got.msg != "routed" {
				t.Errorf("%s emitted message %q, want %q", tc.name, got.msg, "routed")
			}
		})
	}
}

// TestLoggerLevelRouting pins the same mapping for the Logger returned by With,
// which is a second, independently written copy of the same twelve methods.
func TestLoggerLevelRouting(t *testing.T) {
	for _, tc := range []struct {
		name  string
		level Level
		log   func(l Logger, msg string)
	}{
		{name: "Debug", level: LevelDebug, log: func(l Logger, m string) { l.Debug(m) }},
		{name: "Info", level: LevelInfo, log: func(l Logger, m string) { l.Info(m) }},
		{name: "Warn", level: LevelWarn, log: func(l Logger, m string) { l.Warn(m) }},
		{name: "Error", level: LevelError, log: func(l Logger, m string) { l.Error(m) }},

		{name: "DebugContext", level: LevelDebug, log: func(l Logger, m string) { l.DebugContext(context.Background(), m) }},
		{name: "InfoContext", level: LevelInfo, log: func(l Logger, m string) { l.InfoContext(context.Background(), m) }},
		{name: "WarnContext", level: LevelWarn, log: func(l Logger, m string) { l.WarnContext(context.Background(), m) }},
		{name: "ErrorContext", level: LevelError, log: func(l Logger, m string) { l.ErrorContext(context.Background(), m) }},

		{name: "Debugf", level: LevelDebug, log: func(l Logger, m string) { l.Debugf("%s", m) }},
		{name: "Infof", level: LevelInfo, log: func(l Logger, m string) { l.Infof("%s", m) }},
		{name: "Warnf", level: LevelWarn, log: func(l Logger, m string) { l.Warnf("%s", m) }},
		{name: "Errorf", level: LevelError, log: func(l Logger, m string) { l.Errorf("%s", m) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := installRecorder(t)
			tc.log(With(WithName("derived")), "routed")

			got := rec.last(t)
			if got.level != tc.level {
				t.Errorf("%s emitted level %d, want %d", tc.name, got.level, tc.level)
			}
			if got.msg != "routed" {
				t.Errorf("%s emitted message %q, want %q", tc.name, got.msg, "routed")
			}
		})
	}
}

type ctxKey struct{}

// TestContextVariantsForwardTheCallerContext pins that the *Context functions
// hand the caller's context to the backend rather than a fresh background one.
// A backend that enriches entries from context — the way WrapBackendWithContextMerge
// injects trace and request identifiers — produces entries with those fields
// silently missing if the context is dropped on the way through.
func TestContextVariantsForwardTheCallerContext(t *testing.T) {
	for _, tc := range []struct {
		name string
		log  func(ctx context.Context)
	}{
		{name: "DebugContext", log: func(ctx context.Context) { DebugContext(ctx, "m") }},
		{name: "InfoContext", log: func(ctx context.Context) { InfoContext(ctx, "m") }},
		{name: "WarnContext", log: func(ctx context.Context) { WarnContext(ctx, "m") }},
		{name: "ErrorContext", log: func(ctx context.Context) { ErrorContext(ctx, "m") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := installRecorder(t)
			ctx := context.WithValue(context.Background(), ctxKey{}, "trace-1")
			tc.log(ctx)

			if got := rec.last(t).ctx.Value(ctxKey{}); got != "trace-1" {
				t.Fatalf("the caller context did not reach the backend: got value %v", got)
			}
		})
	}
}

// TestNonContextVariantsPassAUsableContext pins that the context-free functions
// still hand the backend a non-nil context. Backends are entitled to use it —
// contextMergeBackend calls its extractor with it unconditionally — so a nil
// here turns an ordinary log line into a nil dereference inside the backend.
func TestNonContextVariantsPassAUsableContext(t *testing.T) {
	for _, tc := range []struct {
		name string
		log  func()
	}{
		{name: "Info", log: func() { Info("m") }},
		{name: "Infof", log: func() { Infof("m") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := installRecorder(t)
			tc.log()

			if rec.last(t).ctx == nil {
				t.Fatal("the backend received a nil context")
			}
		})
	}
}

// TestAttrsReachTheBackend pins that structured fields survive the facade.
func TestAttrsReachTheBackend(t *testing.T) {
	rec := installRecorder(t)
	Error("request failed", String("route", "/v1/users"), Int("status", 500))

	got := rec.last(t)
	if len(got.attrs) != 2 {
		t.Fatalf("backend received %d attrs, want 2", len(got.attrs))
	}
	if got.attrs[0].Key != "route" || got.attrs[0].Value.String() != "/v1/users" {
		t.Errorf("first attr = %v, want route=/v1/users", got.attrs[0])
	}
	if got.attrs[1].Key != "status" || got.attrs[1].Value.Int64() != 500 {
		t.Errorf("second attr = %v, want status=500", got.attrs[1])
	}
}

// TestFormatVariantsApplyTheFormat pins that the printf-style functions actually
// format. They deliberately forward no attrs, so the arguments only survive if
// they were interpolated into the message.
func TestFormatVariantsApplyTheFormat(t *testing.T) {
	rec := installRecorder(t)
	Warnf("user %d retried %s", 42, "twice")

	got := rec.last(t)
	if got.msg != "user 42 retried twice" {
		t.Fatalf("message = %q, want the formatted string", got.msg)
	}
	if len(got.attrs) != 0 {
		t.Fatalf("formatted call forwarded %d attrs, want none", len(got.attrs))
	}
}

// TestWithDerivesFromTheCurrentBackend pins that With builds on whichever backend
// is installed at the moment it is called, and passes the options down.
func TestWithDerivesFromTheCurrentBackend(t *testing.T) {
	rec := installRecorder(t)

	derived := With(WithName("svc"), WithAttrs(map[string]any{"region": "us"}))
	if rec.withOptions != 2 {
		t.Fatalf("backend received %d options, want 2", rec.withOptions)
	}

	derived.Info("through the derived logger")
	if rec.last(t).msg != "through the derived logger" {
		t.Fatal("the derived logger did not write to the installed backend")
	}
	if derived.Backend() == nil {
		t.Fatal("Backend() must expose the underlying backend for bridge adapters")
	}
}

// TestSyncReportsBackendFailure pins that Sync is a real signal rather than a
// swallowed one: a caller flushing before exit must learn that the flush failed.
func TestSyncReportsBackendFailure(t *testing.T) {
	rec := installRecorder(t)
	want := errors.New("disk full")
	rec.syncErr = want

	if err := Sync(); !errors.Is(err, want) {
		t.Fatalf("Sync() = %v, want %v", err, want)
	}

	rec.syncErr = nil
	if err := Sync(); err != nil {
		t.Fatalf("Sync() = %v, want nil", err)
	}
}

// TestLoggerFallsBackWhenUninitialized pins that logging before any
// initialization — an init() in another package, or a panic handler running
// during startup — writes somewhere instead of dereferencing a nil logger.
func TestLoggerFallsBackWhenUninitialized(t *testing.T) {
	prev := std.Load()
	t.Cleanup(func() { std.Store(prev) })

	std.Store(nil)
	Info("logged before initialization")

	if std.Load() == nil {
		t.Fatal("the fallback logger was not installed")
	}
}

// captureBackend records the messages it is asked to log and whether it was
// closed, the two facts the InitWithBackends and Close-forwarding tests assert.
type captureBackend struct {
	entries []string
	closed  bool
}

func (c *captureBackend) Log(_ context.Context, _ Level, msg string, _ ...Attr) {
	c.entries = append(c.entries, msg)
}

func (c *captureBackend) Sync() error { return nil }

func (c *captureBackend) With(...Option) Backend { return c }

func (c *captureBackend) Close() error {
	c.closed = true
	return nil
}

// TestInitWithBackendsDoesNotCloseOldBackend pins the ownership contract: the
// caller constructs backends, so a swap must leave the previous one usable.
func TestInitWithBackendsDoesNotCloseOldBackend(t *testing.T) {
	original := logger()
	t.Cleanup(func() { std.Store(original) })

	old := &closableBackend{}
	InitWithBackends(old)
	InitWithBackends(&closableBackend{})

	if old.closed != 0 {
		t.Errorf("previous backend was closed %d times, want 0", old.closed)
	}
	// Still usable by whoever kept the reference.
	old.Log(context.Background(), LevelInfo, "still alive")
}

// TestInitWithBackendsKeepsCurrentOnEmpty pins that a configuration mistake
// cannot silence the process. InitWithBackends returns nothing, so a caller
// whose backend builder returned nil on one branch had no way to notice that
// every later log line was being discarded — and that branch is exactly the one
// taken when the configuration is already wrong.
func TestInitWithBackendsKeepsCurrentOnEmpty(t *testing.T) {
	original := logger()
	t.Cleanup(func() { std.Store(original) })
	restore := &captureBackend{}
	InitWithBackends(restore)

	for _, tc := range []struct {
		name     string
		backends []Backend
	}{
		{name: "no arguments"},
		{name: "single nil", backends: []Backend{nil}},
		{name: "all nil", backends: []Backend{nil, nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			InitWithBackends(tc.backends...)

			before := len(restore.entries)
			Error("still logging")
			if len(restore.entries) != before+1 {
				t.Fatal("the installed backend was replaced by a silent one")
			}
		})
	}
}

// TestInitWithBackendsAcceptsExplicitNop pins the escape hatch: discarding logs
// on purpose must still work, and must not be mistaken for the empty case.
func TestInitWithBackendsAcceptsExplicitNop(t *testing.T) {
	original := logger()
	t.Cleanup(func() { std.Store(original) })
	seen := &captureBackend{}
	InitWithBackends(seen)
	InitWithBackends(NewNopBackend())

	before := len(seen.entries)
	Error("discarded")
	if len(seen.entries) != before {
		t.Fatal("NewNopBackend() must replace the current backend")
	}
}

// Deeply nested and cyclic types for Objective 2

type recursiveNode struct {
	Next *recursiveNode
	Val  int
}

type panickingStringer struct {
	msg string
}

func (p panickingStringer) String() string {
	panic("panickingStringer: " + p.msg)
}

type deepStruct struct {
	Inner any
	Level int
}

// TestStressDeeplyNestedAndCyclicFormatting satisfies Objective 2:
// pass deeply nested (depth > 32) and cyclic structs, panicking Stringers,
// verifying clean fallback without process crash.
func TestStressDeeplyNestedAndCyclicFormatting(t *testing.T) {
	backends := []struct {
		name string
		b    Backend
	}{
		{name: "StdioBackend", b: NewStdioBackend()},
		{name: "NopBackend", b: NewNopBackend()},
		{name: "MultiBackend", b: NewMultiBackend(NewStdioBackend(), NewNopBackend())},
	}

	for _, tc := range backends {
		t.Run(tc.name, func(t *testing.T) {
			// 1. Deeply nested structure > 32 levels (e.g. depth 50)
			var root any = "bottom"
			for d := 50; d >= 1; d-- {
				root = deepStruct{Inner: root, Level: d}
			}

			// Must not crash or stack overflow
			tc.b.Log(context.Background(), LevelInfo, "testing deep struct", Any("deep", root))

			// Deeply nested slice
			var deepSlice any = "leaf"
			for range 40 {
				deepSlice = []any{deepSlice}
			}
			tc.b.Log(context.Background(), LevelInfo, "testing deep slice", Any("deep_slice", deepSlice))

			// Deeply nested map
			var deepMap any = "leaf"
			for range 40 {
				deepMap = map[string]any{"next": deepMap}
			}
			tc.b.Log(context.Background(), LevelInfo, "testing deep map", Any("deep_map", deepMap))

			// 2. Cyclic pointer structures
			nodeA := &recursiveNode{Val: 1}
			nodeB := &recursiveNode{Val: 2}
			nodeA.Next = nodeB
			nodeB.Next = nodeA // cycle

			tc.b.Log(context.Background(), LevelInfo, "testing cyclic pointer", Any("cyclic_ptr", nodeA))

			// Self-referential pointer
			selfNode := &recursiveNode{Val: 42}
			selfNode.Next = selfNode
			tc.b.Log(context.Background(), LevelInfo, "testing self pointer", Any("self_ptr", selfNode))

			// Cyclic slice
			cyclicSlice := make([]any, 1)
			cyclicSlice[0] = cyclicSlice
			tc.b.Log(context.Background(), LevelInfo, "testing cyclic slice", Any("cyclic_slice", cyclicSlice))

			// Cyclic map
			cyclicMap := make(map[string]any)
			cyclicMap["self"] = cyclicMap
			tc.b.Log(context.Background(), LevelInfo, "testing cyclic map", Any("cyclic_map", cyclicMap))

			// 3. Panicking Stringer
			ps := panickingStringer{msg: "simulated boom"}
			tc.b.Log(context.Background(), LevelError, "testing panicking stringer", Any("stringer", ps))

			// Nested panicking Stringer
			nestedPS := map[string]any{
				"payload": []any{
					panickingStringer{msg: "nested boom 1"},
					panickingStringer{msg: "nested boom 2"},
				},
			}
			tc.b.Log(context.Background(), LevelError, "testing nested panicking stringer", Any("nested", nestedPS))
		})
	}
}

// TestStressGlobalLoggerFormatting tests that global helper methods also survive
// cyclic and panicking structures without crashing.
func TestStressGlobalLoggerFormatting(t *testing.T) {
	orig := logger()
	defer func() { std.Store(orig) }()

	InitWithBackends(NewStdioBackend())

	// Cyclic map
	m := map[string]any{}
	m["m"] = m
	Info("global cyclic map", Any("m", m))
	Error("global panicking stringer", Any("ps", panickingStringer{msg: "global boom"}))

	// Derived logger with panicking attribute
	derived := With(WithName("panic_test"))
	derived.Warn("derived panicking stringer", Any("ps", panickingStringer{msg: "derived boom"}))
}

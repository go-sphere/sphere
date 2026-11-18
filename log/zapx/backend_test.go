package zapx

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-sphere/sphere/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestZapLogger(t *testing.T) {
	originalBackend := log.With().Backend()
	originalSlog := slog.Default()
	t.Cleanup(func() {
		log.InitWithBackends(originalBackend)
		slog.SetDefault(originalSlog)
	})
	backend := NewBackend(NewDefaultConfig(), log.AddCaller())

	log.InitWithBackends(backend)
	log.Warn("info")
	log.Warnf("warn %s", "value")
	log.WarnContext(context.Background(), "warn context")
	log.With(log.WithAttrs(map[string]any{
		"extra": "extra value",
	})).WarnContext(context.Background(), "warn", log.String("key", "value"))

	slog.SetDefault(backend.SlogLogger(log.AddCaller()))
	slog.Warn("slog", log.String("key", "value"))
	slog.WarnContext(context.Background(), "slog", log.String("key", "value"))

	backend.ZapLogger().Warn("zap", zap.String("key", "value"))
	backend.zapLogger.With(zap.String("key", "value")).Warn("zap", zap.String("key2", "value2"))
}

type cyclicZapNode struct {
	Next *cyclicZapNode
	Val  int
}

type panickingZapStringer struct {
	msg string
}

func (p panickingZapStringer) String() string {
	panic("panickingZapStringer: " + p.msg)
}

type panickingZapMarshaler struct {
	msg string
}

func (p panickingZapMarshaler) MarshalJSON() ([]byte, error) {
	panic("panickingZapMarshaler: " + p.msg)
}

type deepZapStruct struct {
	Inner any
	Level int
}

func TestZapxStressFormatting(t *testing.T) {
	backend := NewBackend(NewDefaultConfig())
	defer func() { _ = backend.Close() }()

	t.Run("DeeplyNested", func(t *testing.T) {
		var root any = "bottom"
		for d := 50; d >= 1; d-- {
			root = deepZapStruct{Inner: root, Level: d}
		}
		backend.Log(context.Background(), log.LevelInfo, "deep struct in zapx", log.Any("deep", root))
	})

	t.Run("CyclicStructures", func(t *testing.T) {
		nodeA := &cyclicZapNode{Val: 1}
		nodeB := &cyclicZapNode{Val: 2}
		nodeA.Next = nodeB
		nodeB.Next = nodeA

		backend.Log(context.Background(), log.LevelInfo, "cyclic pointer in zapx", log.Any("cyclic", nodeA))

		cyclicMap := make(map[string]any)
		cyclicMap["self"] = cyclicMap
		backend.Log(context.Background(), log.LevelInfo, "cyclic map in zapx", log.Any("cyclic_map", cyclicMap))
	})

	t.Run("PanickingStringerAndMarshaler", func(t *testing.T) {
		backend.Log(context.Background(), log.LevelError, "panicking stringer", log.Any("stringer", panickingZapStringer{msg: "zapx boom"}))
		backend.Log(context.Background(), log.LevelError, "panicking marshaler", log.Any("marshaler", panickingZapMarshaler{msg: "zapx boom"}))
	})
}

func newFileBackend(t *testing.T) *Backend {
	t.Helper()
	conf := NewDefaultConfig()
	conf.Console.Disable = true
	conf.File.FileName = filepath.Join(t.TempDir(), "app.log")
	return NewBackend(conf)
}

func TestBackendCloseReleasesFile(t *testing.T) {
	backend := newFileBackend(t)
	backend.Log(context.Background(), log.LevelInfo, "before close")

	if err := backend.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Close releases the handle; it does not seal the backend. lumberjack reopens
	// on the next write, so a double Close must stay harmless.
	if err := backend.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// TestDerivedBackendCloseIsNoop pins the ownership rule: only the backend that
// opened the writer closes it, so a logger derived through With can never
// release its parent's handle.
func TestDerivedBackendCloseIsNoop(t *testing.T) {
	parent := newFileBackend(t)
	t.Cleanup(func() { _ = parent.Close() })

	derived, ok := parent.With(log.WithAttrs(map[string]any{"scope": "child"})).(*Backend)
	if !ok {
		t.Fatal("With should return a *Backend")
	}
	if derived.file != nil {
		t.Fatal("derived backend must not own the rotating file writer")
	}
	if err := derived.Close(); err != nil {
		t.Fatalf("derived Close: %v", err)
	}
	if parent.file == nil {
		t.Fatal("parent lost ownership of its writer")
	}
}

func TestBackendWithoutFileCloseIsNoop(t *testing.T) {
	backend := NewBackend(NewDefaultConfig())
	if backend.file != nil {
		t.Fatal("a console-only backend must not own a writer")
	}
	if err := backend.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestSyncSucceedsWithConsoleSink pins that Sync is usable as a success signal.
// Syncing os.Stdout issues an fsync the kernel rejects for a pipe or character
// device, which under a container or CI is what stdout always is — so a console
// sink used to make Sync fail every single time and mask a real file-sink flush
// failure reported through the same joined error.
func TestSyncSucceedsWithConsoleSink(t *testing.T) {
	backend := NewBackend(NewDefaultConfig())
	t.Cleanup(func() { _ = backend.Close() })

	if err := backend.Sync(); err != nil {
		t.Fatalf("Sync() with a console sink = %v, want nil", err)
	}
}

// TestSyncReportsFileSinkFailures pins the other half: silencing the console
// sink must not silence a genuine failure from a file sink.
func TestSyncSucceedsWithFileSink(t *testing.T) {
	conf := NewDefaultConfig()
	conf.File.FileName = t.TempDir() + "/app.log"

	backend := NewBackend(conf)
	t.Cleanup(func() { _ = backend.Close() })

	backend.Log(t.Context(), 0, "entry")
	if err := backend.Sync(); err != nil {
		t.Fatalf("Sync() with a writable file sink = %v, want nil", err)
	}
}

// panicMarshaler panics while being encoded, standing in for a value whose
// MarshalJSON or Stringer has a bug of its own.
type panicMarshaler struct{}

func (panicMarshaler) MarshalJSON() ([]byte, error) { panic("marshal boom") }

// TestLogSurvivesPanickingAttr pins that a log statement cannot abort the
// goroutine that made it.
//
// The same value used to crash here while the stdio backend degraded and
// carried on, so swapping backends changed whether an application survived its
// own logging — and logging sits on error paths, the worst moment to raise a
// second failure.
func TestLogSurvivesPanickingAttr(t *testing.T) {
	backend := NewBackend(NewDefaultConfig())
	t.Cleanup(func() { _ = backend.Close() })

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a panicking attribute escaped the backend: %v", r)
		}
	}()
	backend.Log(t.Context(), log.LevelError, "request failed", log.Any("payload", panicMarshaler{}))
}

type cyclicPanicMarshaler struct{}

func (cyclicPanicMarshaler) MarshalJSON() ([]byte, error) {
	m := map[string]any{}
	m["self"] = m
	panic(m)
}

func TestBackendLogSurvivesCyclicPanicInRecover(t *testing.T) {
	var buf bytes.Buffer
	core := zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&buf), zapcore.DebugLevel)
	backend := newBackendFromCore(core, nil, log.WithName("svc"), nil)
	backend.Log(t.Context(), log.LevelError, "request failed", log.Any("payload", cyclicPanicMarshaler{}))
	if !strings.Contains(buf.String(), "attr_error") || !strings.Contains(buf.String(), "unformattable") {
		t.Fatalf("output = %q, want attr_error with unformattable", buf.String())
	}
}

type cyclicPanicStringer struct{}

func (cyclicPanicStringer) String() string {
	m := map[string]any{}
	m["self"] = m
	panic(m)
}

type cyclicPanicError struct{}

func (cyclicPanicError) Error() string {
	m := map[string]any{}
	m["self"] = m
	panic(m)
}

type cyclicValueMarshalJSON struct {
	Self *cyclicValueMarshalJSON `json:"self"`
}

func (c cyclicValueMarshalJSON) MarshalJSON() ([]byte, error) {
	return []byte(strings.Repeat("x", len(c.Self.MarshalHelper()))), nil
}

func (c cyclicValueMarshalJSON) MarshalHelper() string { return "ok" }

type cyclicStringerNode struct {
	next *cyclicStringerNode
}

func (n *cyclicStringerNode) String() string { return n.next.String() } //nolint:staticcheck // intentional cycle to test recursion guard

func TestSlogHandlerWithoutWithStackAtOmitsErrorStacktrace(t *testing.T) {
	var buf bytes.Buffer
	core := zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&buf), zapcore.DebugLevel)
	backend := newBackendFromCore(core)
	backend.SlogLogger().Error("boom")
	if strings.Contains(buf.String(), `"stacktrace"`) {
		t.Fatalf("slog.Error without WithStackAt emitted unexpected stacktrace: %s", buf.String())
	}
}

func TestBackendSurvivesPanickingPresetAttrsAndCyclicValues(t *testing.T) {
	var buf bytes.Buffer
	core := zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&buf), zapcore.DebugLevel)
	backend := newBackendFromCore(core, log.WithAttrs(map[string]any{
		"good":   "yes",
		"broken": panicMarshaler{},
	}))
	derived := backend.With(log.WithAttrs(map[string]any{
		"broken2": panicMarshaler{},
	}))

	cStr := &cyclicStringerNode{}
	cStr.next = cStr
	cJSON := cyclicValueMarshalJSON{}
	cJSON.Self = &cJSON

	derived.Log(t.Context(), log.LevelError, "msg",
		log.Any("cyclic_str", cStr),
		log.Any("cyclic_json", cJSON),
		log.Any("panic_str", cyclicPanicStringer{}),
		log.Err(cyclicPanicError{}),
	)
	backend.SlogLogger(log.WithAttrs(map[string]any{
		"good_slog":   "ok",
		"broken_slog": panicMarshaler{},
	})).Info("slog_ok")

	out := buf.String()
	if !strings.Contains(out, `"good":"yes"`) || !strings.Contains(out, `"good_slog":"ok"`) {
		t.Fatalf("missing surviving preset attrs: %s", out)
	}
	if !strings.Contains(out, "unformattable") {
		t.Fatalf("missing unformattable marker for cyclic values: %s", out)
	}
}

func TestDisableStackAndWithMinLevelAcrossZapxPaths(t *testing.T) {
	var buf bytes.Buffer
	core := zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&buf), zapcore.DebugLevel)
	parent := newBackendFromCore(core, log.WithStackAt(log.LevelError))

	parent.Log(t.Context(), log.LevelError, "parent_err")
	childNoStack := parent.With(log.DisableStack())
	childNoStack.Log(t.Context(), log.LevelError, "child_no_stack")
	childNoStack.(*Backend).SlogLogger().Error("child_slog_no_stack")
	childWarnMin := parent.With(log.WithMinLevel(log.LevelWarn))
	childWarnMin.Log(t.Context(), log.LevelInfo, "filtered_info")
	childWarnMin.Log(t.Context(), log.LevelWarn, "kept_warn")
	parent.SlogLogger(log.DisableStack(), log.WithMinLevel(log.LevelError)).Warn("slog_filtered_warn")

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d log lines, want 4:\n%s", len(lines), out)
	}
	if !strings.Contains(lines[0], `"msg":"parent_err"`) || !strings.Contains(lines[0], `"stacktrace"`) {
		t.Fatalf("parent_err should include stacktrace: %s", lines[0])
	}
	if !strings.Contains(lines[1], `"msg":"child_no_stack"`) || strings.Contains(lines[1], `"stacktrace"`) {
		t.Fatalf("child_no_stack should omit stacktrace: %s", lines[1])
	}
	if !strings.Contains(lines[2], `"msg":"child_slog_no_stack"`) || strings.Contains(lines[2], `"stacktrace"`) {
		t.Fatalf("child_slog_no_stack should omit stacktrace: %s", lines[2])
	}
	if !strings.Contains(lines[3], `"msg":"kept_warn"`) {
		t.Fatalf("kept_warn missing: %s", lines[3])
	}
	if strings.Contains(out, "filtered_info") || strings.Contains(out, "slog_filtered_warn") {
		t.Fatalf("WithMinLevel did not filter lower levels:\n%s", out)
	}

	// NewBackend with log.WithMinLevel(LevelDebug) overrides Config.Level ("info").
	logPath := filepath.Join(t.TempDir(), "debug.log")
	b := NewBackend(Config{
		Console: ConsoleConfig{Disable: true},
		File:    FileConfig{FileName: logPath},
		Level:   "info",
	}, log.WithMinLevel(log.LevelDebug))
	t.Cleanup(func() {
		_ = b.Sync()
		_ = b.Close()
	})
	b.Log(t.Context(), log.LevelDebug, "debug_via_option")
	_ = b.Sync()
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(raw), `"msg":"debug_via_option"`) {
		t.Fatalf("NewBackend(WithMinLevel(LevelDebug)) did not emit debug entry: %s", raw)
	}
}

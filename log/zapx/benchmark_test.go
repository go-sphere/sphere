package zapx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	corelog "github.com/go-sphere/sphere/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type comparisonZapLogger struct {
	*zap.Logger
	sugar *zap.SugaredLogger
}

type loggerComparison struct {
	name       string
	caller     bool
	persistent bool
	global     bool
	raw        func(comparisonZapLogger)
	wrapped    func(corelog.Logger)
}

// comparisonValue uses each backend's lazy structured-value interface.
type comparisonValue struct{}

func (comparisonValue) LogValue() slog.Value {
	return slog.GroupValue(slog.String("method", "GET"), slog.Int("status", 200))
}

func (comparisonValue) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("method", "GET")
	enc.AddInt("status", 200)
	return nil
}

func loggerComparisons() []loggerComparison {
	err := errors.New("upstream timeout")
	return []loggerComparison{
		{
			name:    "Message",
			raw:     func(l comparisonZapLogger) { l.Info("request completed") },
			wrapped: func(l corelog.Logger) { l.Info("request completed") },
		},
		{
			name: "Fields3",
			raw: func(l comparisonZapLogger) {
				l.Info("request completed", zap.String("method", "GET"), zap.Int("status", 200), zap.Duration("duration", 5*time.Millisecond))
			},
			wrapped: func(l corelog.Logger) {
				l.Info("request completed", corelog.String("method", "GET"), corelog.Int("status", 200), corelog.Duration("duration", 5*time.Millisecond))
			},
		},
		{
			name: "GlobalFields3", global: true,
			raw: func(l comparisonZapLogger) {
				l.Info("request completed", zap.String("method", "GET"), zap.Int("status", 200), zap.Duration("duration", 5*time.Millisecond))
			},
			wrapped: func(_ corelog.Logger) {
				corelog.Info("request completed", corelog.String("method", "GET"), corelog.Int("status", 200), corelog.Duration("duration", 5*time.Millisecond))
			},
		},
		{
			name: "Fields10",
			raw: func(l comparisonZapLogger) {
				l.Info("request completed", zap.String("method", "GET"), zap.String("path", "/users"), zap.Int("status", 200),
					zap.Duration("duration", 5*time.Millisecond), zap.String("request_id", "req-123"), zap.Int64("user_id", 42),
					zap.Bool("cached", false), zap.Int("bytes", 1024), zap.String("region", "sg"), zap.Float64("sample", 0.5))
			},
			wrapped: func(l corelog.Logger) {
				l.Info("request completed", corelog.String("method", "GET"), corelog.String("path", "/users"), corelog.Int("status", 200),
					corelog.Duration("duration", 5*time.Millisecond), corelog.String("request_id", "req-123"), corelog.Int64("user_id", 42),
					corelog.Bool("cached", false), corelog.Int("bytes", 1024), corelog.String("region", "sg"), corelog.Float64("sample", 0.5))
			},
		},
		{
			name: "Group",
			raw: func(l comparisonZapLogger) {
				l.Info("request failed", zap.Dict("request", zap.String("method", "GET"), zap.Duration("duration", 5*time.Millisecond), zap.Error(err)))
			},
			wrapped: func(l corelog.Logger) {
				l.Info("request failed", corelog.Group("request", corelog.String("method", "GET"), corelog.Duration("duration", 5*time.Millisecond), corelog.Err(err)))
			},
		},
		{
			name: "Caller", caller: true,
			raw:     func(l comparisonZapLogger) { l.Info("request completed", zap.Int("status", 200)) },
			wrapped: func(l corelog.Logger) { l.Info("request completed", corelog.Int("status", 200)) },
		},
		{
			name: "WithFields", persistent: true,
			raw:     func(l comparisonZapLogger) { l.Info("request completed", zap.Int("status", 200)) },
			wrapped: func(l corelog.Logger) { l.Info("request completed", corelog.Int("status", 200)) },
		},
		{
			name:    "Formatted",
			raw:     func(l comparisonZapLogger) { l.sugar.Infof("request %s completed with %d", "req-123", 200) },
			wrapped: func(l corelog.Logger) { l.Infof("request %s completed with %d", "req-123", 200) },
		},
		{
			name: "DisabledFields3",
			raw: func(l comparisonZapLogger) {
				l.Debug("request completed", zap.String("method", "GET"), zap.Int("status", 200), zap.Duration("duration", 5*time.Millisecond))
			},
			wrapped: func(l corelog.Logger) {
				l.Debug("request completed", corelog.String("method", "GET"), corelog.Int("status", 200), corelog.Duration("duration", 5*time.Millisecond))
			},
		},
		{
			name: "DisabledGroup",
			raw: func(l comparisonZapLogger) {
				l.Debug("request completed", zap.Dict("request", zap.String("method", "GET"), zap.Int("status", 200)))
			},
			wrapped: func(l corelog.Logger) {
				l.Debug("request completed", corelog.Group("request", corelog.String("method", "GET"), corelog.Int("status", 200)))
			},
		},
		{
			name:    "DisabledLazyValue",
			raw:     func(l comparisonZapLogger) { l.Debug("request completed", zap.Object("request", comparisonValue{})) },
			wrapped: func(l corelog.Logger) { l.Debug("request completed", corelog.Any("request", comparisonValue{})) },
		},
		{
			name:    "DisabledFormatted",
			raw:     func(l comparisonZapLogger) { l.sugar.Debugf("request %s completed with %d", "req-123", 200) },
			wrapped: func(l corelog.Logger) { l.Debugf("request %s completed with %d", "req-123", 200) },
		},
	}
}

func newComparisonLogger(w io.Writer, caller bool) *zap.Logger {
	// Match the production file encoder, including duration units and time format.
	cfg := zap.NewProductionEncoderConfig()
	cfg.TimeKey = "timestamp"
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewCore(zapcore.NewJSONEncoder(cfg), zapcore.AddSync(w), zap.InfoLevel)
	return zap.New(core, zap.WithCaller(caller))
}

func (s loggerComparison) rawLogger(w io.Writer) comparisonZapLogger {
	l := newComparisonLogger(w, s.caller)
	if s.persistent {
		l = l.With(zap.String("service", "api"), zap.String("version", "v1"))
	}
	return comparisonZapLogger{Logger: l, sugar: l.Sugar()}
}

func (s loggerComparison) wrappedLogger(w io.Writer) corelog.Logger {
	l := corelog.NewLogger(newBackendWithLogger(newComparisonLogger(w, s.caller), ""))
	if s.persistent {
		l = l.With(corelog.WithAttrs(map[string]any{"service": "api", "version": "v1"}))
	}
	if s.global {
		corelog.InitWithBackends(l.Backend())
	}
	return l
}

// BenchmarkLoggerComparison includes per-call field construction and JSON
// encoding, but excludes sink I/O and one-time logger/With setup. Nothing is
// pre-converted on the wrapped path. The formatted cases use Zap's Sugar API.
func BenchmarkLoggerComparison(b *testing.B) {
	original := corelog.With().Backend()
	b.Cleanup(func() { corelog.InitWithBackends(original) })
	for _, scenario := range loggerComparisons() {
		b.Run(scenario.name, func(b *testing.B) {
			b.Run("Zap", func(b *testing.B) {
				logger := scenario.rawLogger(io.Discard)
				b.ReportAllocs()
				for b.Loop() {
					scenario.raw(logger)
				}
			})
			b.Run("Sphere", func(b *testing.B) {
				logger := scenario.wrappedLogger(io.Discard)
				b.ReportAllocs()
				for b.Loop() {
					scenario.wrapped(logger)
				}
			})
		})
	}
}

// TestLoggerComparisonOutput verifies the benchmark paths do equivalent work
// and prints their actual output with -v. Only time and call-site lines differ.
func TestLoggerComparisonOutput(t *testing.T) {
	original := corelog.With().Backend()
	t.Cleanup(func() { corelog.InitWithBackends(original) })
	for _, scenario := range loggerComparisons() {
		t.Run(scenario.name, func(t *testing.T) {
			var raw, wrapped bytes.Buffer
			scenario.raw(scenario.rawLogger(&raw))
			scenario.wrapped(scenario.wrappedLogger(&wrapped))
			t.Logf("Zap: %s", strings.TrimSpace(raw.String()))
			t.Logf("Sphere: %s", strings.TrimSpace(wrapped.String()))
			if strings.HasPrefix(scenario.name, "Disabled") {
				if raw.Len() != 0 || wrapped.Len() != 0 {
					t.Fatal("disabled log call emitted output")
				}
				return
			}
			entries := make([]map[string]any, 0, 2)
			for _, output := range [][]byte{raw.Bytes(), wrapped.Bytes()} {
				var entry map[string]any
				if err := json.Unmarshal(output, &entry); err != nil {
					t.Fatal(err)
				}
				delete(entry, "timestamp")
				if scenario.caller {
					caller, ok := entry["caller"].(string)
					if !ok || !strings.Contains(caller, "benchmark_test.go:") {
						t.Fatalf("caller = %v, want the benchmark call site", entry["caller"])
					}
					delete(entry, "caller")
				}
				entries = append(entries, entry)
			}
			if !reflect.DeepEqual(entries[0], entries[1]) {
				t.Fatalf("different log content: Zap=%v Sphere=%v", entries[0], entries[1])
			}
		})
	}
}

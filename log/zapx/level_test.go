package zapx

import (
	"log/slog"
	"testing"

	corelog "github.com/go-sphere/sphere/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type countingLogValuer struct {
	calls int
}

func (v *countingLogValuer) LogValue() slog.Value {
	v.calls++
	return slog.StringValue("resolved")
}

func TestDisabledLevelSkipsAttrResolution(t *testing.T) {
	level := zap.NewAtomicLevelAt(zap.InfoLevel)
	core, logs := observer.New(level)
	backend := newBackendFromCore(core)
	value := &countingLogValuer{}

	backend.Log(t.Context(), corelog.LevelDebug, "disabled", corelog.Any("value", value))
	if value.calls != 0 || logs.Len() != 0 {
		t.Fatalf("disabled entry: resolutions=%d, entries=%d; want both zero", value.calls, logs.Len())
	}

	level.SetLevel(zap.DebugLevel)
	backend.Log(t.Context(), corelog.LevelDebug, "enabled", corelog.Any("value", value))
	if value.calls != 1 || logs.Len() != 1 {
		t.Fatalf("enabled entry: resolutions=%d, entries=%d; want both one", value.calls, logs.Len())
	}
	if got := logs.All()[0].ContextMap()["value"]; got != "resolved" {
		t.Fatalf("value = %v, want resolved", got)
	}
}

// TestOutOfRangeLevelsClampToNearest pins that a level below LevelDebug is
// logged as debug and one above LevelError as error, and that the clamped level
// is what the core filters on.
func TestOutOfRangeLevelsClampToNearest(t *testing.T) {
	for _, tc := range []struct {
		minimum zapcore.Level
		want    []zapcore.Level
	}{
		{minimum: zap.DebugLevel, want: []zapcore.Level{zap.DebugLevel, zap.ErrorLevel}},
		{minimum: zap.InfoLevel, want: []zapcore.Level{zap.ErrorLevel}},
	} {
		t.Run(tc.minimum.String(), func(t *testing.T) {
			core, logs := observer.New(tc.minimum)
			backend := newBackendFromCore(core)
			value := &countingLogValuer{}
			for _, level := range []corelog.Level{-1, 127} {
				backend.Log(t.Context(), level, "out of range", corelog.Any("value", value))
			}
			if value.calls != len(tc.want) || logs.Len() != len(tc.want) {
				t.Fatalf("resolutions=%d, entries=%d; want both %d", value.calls, logs.Len(), len(tc.want))
			}
			for i, entry := range logs.All() {
				if entry.Level != tc.want[i] {
					t.Errorf("entry %d level = %v, want %v", i, entry.Level, tc.want[i])
				}
			}
		})
	}
}

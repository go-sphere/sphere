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
	backend := newBackendWithLogger(zap.New(core), "")
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

func TestLevelFilterPreservesUnknownLevelFallback(t *testing.T) {
	for _, minimum := range []zapcore.Level{zap.InfoLevel, zap.WarnLevel} {
		t.Run(minimum.String(), func(t *testing.T) {
			core, logs := observer.New(minimum)
			backend := newBackendWithLogger(zap.New(core), "")
			value := &countingLogValuer{}
			for _, level := range []corelog.Level{-1, 127} {
				backend.Log(t.Context(), level, "unknown", corelog.Any("value", value))
			}
			want := 0
			if minimum == zap.InfoLevel {
				want = 2
			}
			if value.calls != want || logs.Len() != want {
				t.Fatalf("resolutions=%d, entries=%d; want both %d", value.calls, logs.Len(), want)
			}
			for _, entry := range logs.All() {
				if entry.Level != zap.InfoLevel {
					t.Errorf("level = %v, want info", entry.Level)
				}
			}
		})
	}
}

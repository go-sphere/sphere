package log

import (
	"log/slog"
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestNewSlogLogger(t *testing.T) {
	slog.SetDefault(NewSlogLogger(NewDefaultConfig(), AddCaller()))
	slog.Debug("debug message")
	slog.Info("info message")
	slog.Warn("warn message")
	slog.Error("error message")

	log, err := WarpAsSlog(With(), AddCaller())
	slog.SetDefault(log)
	if err != nil {
		t.Fatalf("failed to set slog default logger: %v", err)
	}
	slog.Debug("debug message")
	slog.Info("info message", slog.Any("key", "value"))
	slog.Warn("warn message")
	slog.Error("error message")

	Debug("debug message")
	Info("info message", "key", "value")
	Warn("warn message")
	Error("error message")
}

func TestNewSlogLoggerNilConfig(t *testing.T) {
	logger := NewSlogLogger(nil)
	if logger == nil {
		t.Fatal("logger should not be nil")
	}
	logger.Info("info message")
}

func TestZapLevelToSlogLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    zapcore.Level
		expected slog.Level
	}{
		{name: "debug", input: zapcore.DebugLevel, expected: slog.LevelDebug},
		{name: "info", input: zapcore.InfoLevel, expected: slog.LevelInfo},
		{name: "warn", input: zapcore.WarnLevel, expected: slog.LevelWarn},
		{name: "error", input: zapcore.ErrorLevel, expected: slog.LevelError},
		{name: "dpanic", input: zapcore.DPanicLevel, expected: slog.LevelError + 1},
		{name: "panic", input: zapcore.PanicLevel, expected: slog.LevelError + 2},
		{name: "fatal", input: zapcore.FatalLevel, expected: slog.LevelError + 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := zapLevelToSlogLevel(tt.input); got != tt.expected {
				t.Fatalf("unexpected level: got %v want %v", got, tt.expected)
			}
		})
	}
}

func TestMapToSlogAttrsStableOrder(t *testing.T) {
	attrs := map[string]any{
		"z": 1,
		"a": 2,
		"m": 3,
	}
	got := mapToSlogAttrs(attrs)
	if len(got) != 3 {
		t.Fatalf("unexpected attrs length: got %d want %d", len(got), 3)
	}
	if got[0].Key != "a" || got[1].Key != "m" || got[2].Key != "z" {
		t.Fatalf("unexpected order: got [%s %s %s] want [a m z]", got[0].Key, got[1].Key, got[2].Key)
	}
}

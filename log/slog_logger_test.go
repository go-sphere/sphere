package log

import (
	"log/slog"
	"testing"
)

func TestNewSlogLogger(t *testing.T) {
	slog.SetDefault(NewSlogLogger(NewDefaultConfig(), AddCaller()))
	slog.Debug("debug message")
	slog.Info("info message")
	slog.Warn("warn message")
	slog.Error("error message")

	log, err := UnwarpSlogLogger(With(), AddCaller())
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

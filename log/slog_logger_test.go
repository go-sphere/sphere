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
}

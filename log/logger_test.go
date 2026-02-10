package log

import (
	"context"
	"testing"
)

func TestLogger(t *testing.T) {
	InitWithBackends(NewStdioBackend())
	Debug("debug")
	Info("info")
	With(WithAttrs(map[string]any{
		"extra": "extra value",
	})).Warn("warn", String("key", "value"))
	Error("error")
	_ = Sync()
}

func TestContextLogging(t *testing.T) {
	InitWithBackends(NewStdioBackend())
	InfoContext(context.Background(), "context info should not panic")
}

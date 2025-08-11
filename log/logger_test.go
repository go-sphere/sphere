package log

import (
	"fmt"
	"log/slog"
	"testing"
)

func TestLogger(t *testing.T) {
	Init(&Options{
		Level: "debug",
	}, nil)
	Debug("debug")
	Info("info")
	Warn("warn")
	Error("error", slog.Any("error", fmt.Errorf("test error")))
	_ = Sync()
}

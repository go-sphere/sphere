package log

import (
	"fmt"
	"testing"
)

func TestLogger(t *testing.T) {
	Init(&Config{
		Level: "debug",
	}, map[string]any{
		"version": "test",
	})
	Debug("debug")
	Info("info")
	With(WithAttrs(map[string]any{
		"extra": "extra value",
	})).Warn("warn", "key", "value")
	Error("error", Err(fmt.Errorf("test error")))
	_ = Sync()
}

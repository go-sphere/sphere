package log

import (
	"fmt"
	"testing"
)

func TestLogger(t *testing.T) {
	Init(&Config{
		Level: "debug",
	}, nil)
	Debug("debug")
	Info("info")
	Warn("warn", "key", "value")
	Error("error", Err(fmt.Errorf("test error")))
	_ = Sync()
}

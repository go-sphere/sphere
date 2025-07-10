package log

import (
	"testing"
)

func TestLogger(t *testing.T) {
	Init(&Options{
		Level: "debug",
	})
	Debug("debug")
	Info("info")
	Warn("warn")
	Error("error")
	_ = Sync()
}

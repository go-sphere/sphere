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

func TestMapToZapFieldsStableOrder(t *testing.T) {
	fields := mapToZapFields(map[string]any{
		"z": 1,
		"a": 2,
		"m": 3,
	})
	if len(fields) != 3 {
		t.Fatalf("unexpected fields length: got %d want %d", len(fields), 3)
	}
	if fields[0].Key != "a" || fields[1].Key != "m" || fields[2].Key != "z" {
		t.Fatalf("unexpected order: got [%s %s %s] want [a m z]", fields[0].Key, fields[1].Key, fields[2].Key)
	}
}

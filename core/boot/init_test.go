package boot

import (
	"testing"
	"time"
)

func TestInitTimezone(t *testing.T) {
	t.Run("valid timezone updates time.Local", func(t *testing.T) {
		orig := time.Local
		defer func() {
			time.Local = orig
			_ = InitTimezone(DefaultTimezone)
		}()

		err := InitTimezone("UTC")
		if err != nil {
			t.Fatalf("InitTimezone('UTC') failed: %v", err)
		}
		if time.Local.String() != "UTC" {
			t.Fatalf("expected time.Local to be 'UTC', got %s", time.Local.String())
		}
	})

	t.Run("invalid timezone returns error and leaves location unchanged", func(t *testing.T) {
		orig := time.Local
		defer func() {
			time.Local = orig
			_ = InitTimezone(DefaultTimezone)
		}()

		err := InitTimezone("Invalid/Nonexistent_Timezone_12345")
		if err == nil {
			t.Fatal("expected error for invalid timezone, got nil")
		}
		if time.Local != orig {
			t.Fatalf("time.Local changed after invalid timezone: got %v, want %v", time.Local, orig)
		}
	})
}

func TestInitVersionPrinter(t *testing.T) {
	orig := versionPrinter
	defer func() {
		versionPrinter = orig
	}()

	var printed string
	InitVersionPrinter(func(v string) {
		printed = v
	})

	versionPrinter("v1.2.3")
	if printed != "v1.2.3" {
		t.Fatalf("expected 'v1.2.3', got %q", printed)
	}
}

package boot

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestInitTimezone(t *testing.T) {
	t.Run("valid timezone updates time.Local", func(t *testing.T) {
		orig := time.Local
		t.Cleanup(func() { time.Local = orig })

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
		t.Cleanup(func() { time.Local = orig })

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

// TestImportLeavesTimezoneAlone pins that importing boot has no timezone side
// effect. The package used to call InitTimezone(DefaultTimezone) from init, so
// every binary that linked it ran in Asia/Shanghai whatever the host said. The
// check runs in a child process because init has already run in this one.
func TestImportLeavesTimezoneAlone(t *testing.T) {
	if os.Getenv("BOOT_TZ_CHILD") == "1" {
		if got := os.Getenv("TZ"); got != "UTC" {
			fmt.Printf("TZ = %q, want %q\n", got, "UTC")
			os.Exit(1)
		}
		if got := time.Local.String(); got != "UTC" {
			fmt.Printf("time.Local = %q, want %q\n", got, "UTC")
			os.Exit(1)
		}
		os.Exit(0)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestImportLeavesTimezoneAlone$")
	cmd.Env = append(os.Environ(), "BOOT_TZ_CHILD=1", "TZ=UTC")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child process: %v\n%s", err, out)
	}
}

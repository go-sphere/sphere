package zapx

import (
	"runtime"
	"testing"

	corelog "github.com/go-sphere/sphere/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// TestCoreCallerOffsetReportsUserCallSite pins coreCallerOffset: with
// AddCaller enabled, an entry logged through the core facade must be
// attributed to the user's call site, not to a facade or backend frame.
func TestCoreCallerOffsetReportsUserCallSite(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core, zap.AddCaller())
	lg := corelog.NewLogger(newBackendWithLogger(logger, ""))

	_, file, line, _ := runtime.Caller(0)
	lg.Info("probe") // must be reported as file:line+1

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	caller := entries[0].Caller
	if !caller.Defined {
		t.Fatal("caller not recorded")
	}
	if caller.File != file || caller.Line != line+1 {
		t.Fatalf("caller = %s:%d, want %s:%d", caller.File, caller.Line, file, line+1)
	}
}

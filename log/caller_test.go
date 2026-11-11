package log_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/go-sphere/sphere/log"
)

// wrapperShapes covers the backend shapes callers install: the backend alone,
// inside a MultiBackend, inside the context-merge wrapper, and nested through
// both. The reported caller must be the same for all of them.
func wrapperShapes() map[string]func(log.Backend) log.Backend {
	extract := func(context.Context) []log.Attr { return []log.Attr{log.String("trace_id", "t")} }
	return map[string]func(log.Backend) log.Backend{
		"direct": func(b log.Backend) log.Backend { return b },
		"multi":  func(b log.Backend) log.Backend { return log.NewMultiBackend(b, log.NewNopBackend()) },
		"merge":  func(b log.Backend) log.Backend { return log.WrapBackendWithContextMerge(b, extract) },
		"nested": func(b log.Backend) log.Backend {
			inner := log.NewMultiBackend(b, log.NewNopBackend())
			return log.NewMultiBackend(log.WrapBackendWithContextMerge(inner, extract), log.NewNopBackend())
		},
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	fn()
	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	return buf.String()
}

// TestStdioCallerIsTheUserCallSite pins that the stdio backend reports the
// line that called the logger, however many log-package frames sit between.
func TestStdioCallerIsTheUserCallSite(t *testing.T) {
	for name, wrap := range wrapperShapes() {
		t.Run(name, func(t *testing.T) {
			lg := log.NewLogger(wrap(log.NewStdioBackend(log.AddCaller())))
			var line int
			out := captureStdout(t, func() {
				_, _, line, _ = runtime.Caller(0)
				lg.InfoContext(context.Background(), "probe")
			})
			want := "caller=caller_test.go:" + strconv.Itoa(line+1) + " "
			if !strings.Contains(out, want) {
				t.Fatalf("output = %q, want %q", out, want)
			}
		})
	}
}

// TestStdioCallerThroughGlobalFacade covers the package-level functions, which
// reach the backend without a Logger method frame.
func TestStdioCallerThroughGlobalFacade(t *testing.T) {
	original := log.With().Backend()
	t.Cleanup(func() { log.InitWithBackends(original) })
	for name, wrap := range wrapperShapes() {
		t.Run(name, func(t *testing.T) {
			log.InitWithBackends(wrap(log.NewStdioBackend(log.AddCaller())))
			var line int
			out := captureStdout(t, func() {
				_, _, line, _ = runtime.Caller(0)
				log.Info("probe")
			})
			want := "caller=caller_test.go:" + strconv.Itoa(line+1) + " "
			if !strings.Contains(out, want) {
				t.Fatalf("output = %q, want %q", out, want)
			}
		})
	}
}

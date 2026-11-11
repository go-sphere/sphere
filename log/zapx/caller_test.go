package zapx_test

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/log/zapx"
)

// newFileBackend returns a backend that writes JSON entries to a temp file
// only, and a function that syncs it and returns the decoded entries.
func newFileBackend(t *testing.T, options ...log.Option) (*zapx.Backend, func() []map[string]any) {
	t.Helper()
	conf := zapx.NewDefaultConfig()
	conf.Level = "debug"
	conf.Console.Disable = true
	conf.File.FileName = filepath.Join(t.TempDir(), "app.log")
	backend := zapx.NewBackend(conf, options...)
	t.Cleanup(func() { _ = backend.Close() })
	return backend, func() []map[string]any {
		t.Helper()
		_ = backend.Sync()
		f, err := os.Open(conf.File.FileName)
		if err != nil {
			t.Fatalf("open log file: %v", err)
		}
		defer func() { _ = f.Close() }()
		var entries []map[string]any
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for scanner.Scan() {
			var entry map[string]any
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
				t.Fatalf("decode %q: %v", scanner.Text(), err)
			}
			entries = append(entries, entry)
		}
		return entries
	}
}

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

// TestCallerIsTheUserCallSite pins that entries logged through the core API
// carry the user's call site, however many log-package frames sit between.
func TestCallerIsTheUserCallSite(t *testing.T) {
	for name, wrap := range wrapperShapes() {
		t.Run(name, func(t *testing.T) {
			backend, entries := newFileBackend(t, log.AddCaller())
			lg := log.NewLogger(wrap(backend))
			_, _, line, _ := runtime.Caller(0)
			lg.InfoContext(context.Background(), "probe")

			got := entries()
			if len(got) != 1 {
				t.Fatalf("entries = %v, want 1", got)
			}
			if want := "zapx/caller_test.go:" + strconv.Itoa(line+1); got[0]["caller"] != want {
				t.Fatalf("caller = %v, want %s", got[0]["caller"], want)
			}
		})
	}
}

// TestCallerSurvivesPanickingAttr pins that the replacement entry written after
// an attribute panics keeps the user's call site.
func TestCallerSurvivesPanickingAttr(t *testing.T) {
	backend, entries := newFileBackend(t, log.AddCaller())
	lg := log.NewLogger(log.NewMultiBackend(backend, log.NewNopBackend()))
	_, _, line, _ := runtime.Caller(0)
	lg.Info("probe", log.Any("bad", panickingMarshaler{}))

	got := entries()
	if len(got) != 1 || got[0]["attr_error"] == nil {
		t.Fatalf("entries = %v, want one attr_error entry", got)
	}
	if want := "zapx/caller_test.go:" + strconv.Itoa(line+1); got[0]["caller"] != want {
		t.Fatalf("caller = %v, want %s", got[0]["caller"], want)
	}
}

// TestCallerDisabledByDefault pins that no caller is recorded without
// AddCaller.
func TestCallerDisabledByDefault(t *testing.T) {
	backend, entries := newFileBackend(t)
	log.NewLogger(backend).Info("probe")
	if got := entries(); len(got) != 1 || got[0]["caller"] != nil {
		t.Fatalf("entries = %v, want one entry without caller", got)
	}
}

type panickingMarshaler struct{}

func (panickingMarshaler) MarshalJSON() ([]byte, error) { panic("boom") }

// TestSlogLoggerInheritsCallerAndStack pins that the slog bridge carries the
// backend's AddCaller and WithStackAt settings, so boot.WithLoggerBackend,
// which calls SlogLogger with no options, keeps them.
func TestSlogLoggerInheritsCallerAndStack(t *testing.T) {
	backend, entries := newFileBackend(t, log.AddCaller(), log.WithStackAt(log.LevelWarn))
	logger := backend.SlogLogger()
	_, _, line, _ := runtime.Caller(0)
	logger.Warn("probe")

	got := entries()
	if len(got) != 1 {
		t.Fatalf("entries = %v, want 1", got)
	}
	if want := "zapx/caller_test.go:" + strconv.Itoa(line+1); got[0]["caller"] != want {
		t.Errorf("caller = %v, want %s", got[0]["caller"], want)
	}
	if got[0]["stacktrace"] == nil {
		t.Errorf("entry = %v, want a stacktrace", got[0])
	}
}

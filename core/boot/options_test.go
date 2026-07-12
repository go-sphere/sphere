package boot

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/log/zapx"
)

func TestWithLoggerInitAddsVersionAttribute(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "app.log")
	conf := zapx.NewDefaultConfig()
	conf.Console.Disable = true
	conf.File.FileName = logFile

	opts := newOptions(WithLoggerInit("v1.2.3", conf))
	if err := runHooks(context.Background(), opts.beforeStart, "beforeStart"); err != nil {
		t.Fatalf("run before-start hooks: %v", err)
	}
	t.Cleanup(func() {
		_ = runHooks(context.Background(), opts.afterStop, "afterStop")
		log.InitWithBackends(log.NewStdioBackend())
	})

	log.Info("versioned log entry")
	if err := runHooks(context.Background(), opts.afterStop, "afterStop"); err != nil {
		t.Fatalf("run after-stop hooks: %v", err)
	}

	raw, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	var entry map[string]any
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("decode log entry %q: %v", raw, err)
	}
	if got := entry["version"]; got != "v1.2.3" {
		t.Fatalf("version attribute = %v, want v1.2.3", got)
	}
}

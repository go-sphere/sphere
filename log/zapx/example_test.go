package zapx_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/log/zapx"
)

// Write JSON entries to a rotating file, then Sync and Close at shutdown.
// The console sink is disabled here only to keep the output deterministic.
func ExampleNewBackend() {
	dir, err := os.MkdirTemp("", "zapx-example")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()

	conf := zapx.NewDefaultConfig()
	conf.Console.Disable = true
	conf.File.FileName = filepath.Join(dir, "app.log")

	backend := zapx.NewBackend(conf, log.WithName("api"))
	logger := log.NewLogger(backend)
	logger.Debug("dropped: below the default info level")
	logger.Info("ready", log.Int("port", 8080))

	if err := backend.Sync(); err != nil {
		fmt.Println("sync:", err)
	}
	if err := backend.Close(); err != nil {
		fmt.Println("close:", err)
	}

	f, err := os.Open(conf.File.FileName)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(entry["level"], entry["logger"], entry["msg"], entry["port"])
	}

	// Output: info api ready 8080
}

// Package zapx is a [github.com/go-sphere/sphere/log.Backend] on
// go.uber.org/zap: colored console output on stdout and optional JSON file
// output with lumberjack rotation.
//
// Construct a [Backend] with [NewBackend] and a [Config] (start from
// [NewDefaultConfig]), install it with log.InitWithBackends, and Sync then
// Close it at shutdown. [Backend.SlogLogger] and [Backend.ZapLogger] expose
// the same sinks to code that wants log/slog or zap directly.
//
// # Usage
//
//	import (
//		"github.com/go-sphere/sphere/log"
//		"github.com/go-sphere/sphere/log/zapx"
//	)
//
//	conf := zapx.NewDefaultConfig()
//	conf.File.FileName = "logs/app.log" // empty disables the file sink
//	backend := zapx.NewBackend(conf, log.WithName("api"))
//	log.InitWithBackends(backend)
//	defer func() {
//		_ = backend.Sync()
//		_ = backend.Close()
//	}()
//
//	log.Info("ready", log.Int("port", 8080))
//
// # Ownership and levels
//
// NewBackend owns the file handle; backends derived through With do not.
// Close releases the file, but lumberjack reopens it on the next write, so
// Close does not seal the backend. Close does not flush; call Sync first if
// buffered entries must land.
//
// log.WithMinLevel is ignored; set Config.Level (a zap level string, default
// "info"). An invalid Level falls back to info. FileConfig.MaxSize is in
// megabytes, MaxAge in days, and MaxBackups is a file count.
package zapx

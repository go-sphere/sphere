package telemetry_test

import (
	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/log"
	"github.com/go-sphere/sphere/server/middleware/logger"
	"github.com/go-sphere/sphere/server/middleware/requestid"
	"github.com/go-sphere/sphere/server/middleware/telemetry"
)

// Register tracing outermost, metrics directly inside it, and recovery inside
// the access log.
func Example() {
	var lg log.BaseLogger
	metrics, err := telemetry.NewMetrics()
	if err != nil {
		return
	}
	_ = []httpx.Middleware{
		telemetry.NewTracing(),
		metrics,
		requestid.New(),
		logger.Log(lg),
		logger.RecoveryLog(lg, true),
	}
}

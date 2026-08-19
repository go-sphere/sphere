// Package idgenerator provides distributed unique ID generation using the Snowflake algorithm.
// It supports multiple worker instances with configurable worker IDs to ensure
// globally unique 64-bit integers across distributed systems.
package idgenerator

import (
	"os"
	"strconv"
	"time"

	"github.com/go-sphere/sphere/log"
	"github.com/yitter/idgenerator-go/idgen"
)

// defaultWorkerID is used when WORKER_ID is unset, which is the correct default
// for a single-instance deployment.
const defaultWorkerID uint16 = 1

// parseWorkerID resolves the worker ID from the raw WORKER_ID value. The second
// return value reports whether the raw value was usable, so callers can warn about
// a misconfigured value while staying silent when it is simply unset.
func parseWorkerID(raw string) (uint16, bool) {
	if raw == "" {
		return defaultWorkerID, true
	}
	workerID, err := strconv.ParseUint(raw, 10, 16)
	if err != nil || workerID == 0 {
		return defaultWorkerID, false
	}
	return uint16(workerID), true
}

func init() {
	workerIDRaw := os.Getenv("WORKER_ID")
	workerID, ok := parseWorkerID(workerIDRaw)
	if !ok {
		log.Warn(
			"idgenerator: invalid WORKER_ID, falling back to the default worker ID",
			log.String("worker_id", workerIDRaw),
			log.Int("default", int(defaultWorkerID)),
		)
	}
	options := idgen.NewIdGeneratorOptions(workerID)
	options.BaseTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local).UnixMilli()
	idgen.SetIdGenerator(options)
}

// NextId generates the next unique ID using the global ID generator.
// It returns a 64-bit integer that is globally unique across distributed systems.
func NextId() int64 {
	return idgen.NextId()
}

// NewIdGenerator creates a new ID generator instance with the specified worker ID.
// Each worker should have a unique ID to ensure global uniqueness across multiple instances.
// Returns a function that generates new unique IDs when called.
func NewIdGenerator(workerID uint16) func() int64 {
	options := idgen.NewIdGeneratorOptions(workerID)
	options.BaseTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local).UnixMilli()
	generator := idgen.NewDefaultIdGenerator(options)
	return func() int64 {
		return generator.NewLong()
	}
}

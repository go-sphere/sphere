// Package idgenerator provides distributed unique ID generation using the Snowflake algorithm.
// It supports multiple worker instances with configurable worker IDs to ensure
// globally unique 64-bit integers across distributed systems.
package idgenerator

import (
	"os"
	"strconv"
	"time"

	"github.com/yitter/idgenerator-go/idgen"
)

func init() {
	workerIDRaw := os.Getenv("WORKER_ID")
	workerID, err := strconv.ParseUint(workerIDRaw, 10, 16)
	if err != nil || workerID == 0 {
		workerID = 1
	}
	options := idgen.NewIdGeneratorOptions(uint16(workerID))
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

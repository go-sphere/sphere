// Package idgenerator provides distributed unique ID generation using the Snowflake algorithm.
// It supports multiple worker instances with configurable worker IDs to ensure
// globally unique 64-bit integers across distributed systems.
package idgenerator

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/yitter/idgenerator-go/idgen"
)

// defaultWorkerID is used when WORKER_ID is unset, which is the correct default
// for a single-instance deployment.
const defaultWorkerID uint16 = 1

// maxWorkerID is the largest worker ID the underlying generator accepts with the
// default WorkerIdBitLength of 6 (2^6-1). Zero is a valid worker ID.
const maxWorkerID uint64 = 63

// parseWorkerID resolves the worker ID from the raw WORKER_ID value.
//
// An unset value keeps defaultWorkerID, the correct choice for a single-instance
// deployment. Anything else must parse into [0, maxWorkerID] or the process
// fails: this package promises globally unique IDs, and that promise cannot be
// kept by guessing. Falling back to a fixed ID on a malformed value is the worst
// available outcome — two replicas silently share a worker ID and emit
// colliding IDs, which typically surfaces much later as a duplicate primary key
// or as records attributed to the wrong entity.
//
// Zero is explicitly valid. Rejecting it used to collide pod-0 with pod-1 under
// the standard StatefulSet pattern of deriving WORKER_ID from the pod ordinal.
func parseWorkerID(raw string) (uint16, error) {
	if raw == "" {
		return defaultWorkerID, nil
	}
	workerID, err := strconv.ParseUint(raw, 10, 16)
	if err != nil || workerID > maxWorkerID {
		return 0, fmt.Errorf("idgenerator: invalid WORKER_ID %q: must be an integer in [0, %d]", raw, maxWorkerID)
	}
	return uint16(workerID), nil
}

func init() {
	workerID, err := parseWorkerID(os.Getenv("WORKER_ID"))
	if err != nil {
		panic(err)
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

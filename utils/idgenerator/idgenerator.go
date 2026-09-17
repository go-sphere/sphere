// Package idgenerator produces process-unique int64 IDs via
// yitter/idgenerator-go. It is not Twitter Snowflake (no datacenter bits,
// different layout).
//
// init reads WORKER_ID (valid 0–63). Unset defaults to 1, not 0 — so
// StatefulSet pod-0 must set WORKER_ID=0 explicitly. Malformed values
// panic. BaseTime is the fixed instant baseTimeMillis, never time.Local.
// NextId is the process-global generator; NewIdGenerator(workerID) is
// independent. Unique worker IDs are required across processes sharing a key
// space.
package idgenerator

import (
	"fmt"
	"os"
	"strconv"

	"github.com/yitter/idgenerator-go/idgen"
)

// defaultWorkerID is used when WORKER_ID is unset, which is the correct default
// for a single-instance deployment.
const defaultWorkerID uint16 = 1

// maxWorkerID is the largest worker ID the underlying generator accepts with the
// default WorkerIdBitLength of 6 (2^6-1). Zero is a valid worker ID.
const maxWorkerID uint64 = 63

// baseTimeMillis is the epoch IDs count ticks from: 2023-12-31T10:00:00Z, the
// earliest instant that is 2024-01-01 anywhere on Earth (UTC+14).
//
// It is deliberately not time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local). Which
// zone that evaluates in is decided by import order — boot.InitTimezone assigns
// time.Local from another package's init — and by whether the image ships
// tzdata, so one instant maps to ticks up to 26 hours apart across processes.
// Two processes (or one process across a restart) running the same WORKER_ID
// then issue the same (tick, worker, sequence) triple twice, which is the one
// thing this package promises not to do.
//
// Being the earliest such instant, it also keeps ticks no lower than every value the
// zone-dependent expression could have produced, so a deployment that already
// issued IDs advances its tick range on upgrade. Reusing a worker ID in
// simultaneous processes or rolling back the epoch is still unsafe.
const baseTimeMillis int64 = 1704016800000

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
	options.BaseTime = baseTimeMillis
	idgen.SetIdGenerator(options)
}

// NextId returns the next ID from the process-global generator configured in
// init from WORKER_ID.
func NextId() int64 {
	return idgen.NextId()
}

// NewIdGenerator returns an independent generator for workerID. Unique worker
// IDs are required across processes sharing a key space.
func NewIdGenerator(workerID uint16) func() int64 {
	options := idgen.NewIdGeneratorOptions(workerID)
	options.BaseTime = baseTimeMillis
	generator := idgen.NewDefaultIdGenerator(options)
	return func() int64 {
		return generator.NewLong()
	}
}

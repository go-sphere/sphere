package idgenerator

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"

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
// zone that evaluates in is decided by the host's TZ, by whether the image ships
// tzdata, and by whether main has called boot.InitTimezone by the time it is
// evaluated (before v0.0.7 boot also assigned time.Local from its own package
// init, so import order decided it too), so one instant maps to ticks up to 26
// hours apart across processes.
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
// fails: falling back to a fixed ID would let two replicas silently share a
// worker ID and emit colliding IDs.
//
// Zero is valid, so WORKER_ID can be the StatefulSet pod ordinal.
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

// ErrAlreadyInitialized is returned by Init and InitFromEnv when the one-time
// initialization of the global generator already ran — by an earlier
// Init/InitFromEnv or lazily by NextId — whether or not it succeeded. The
// worker ID of a running generator cannot change: IDs it already issued would
// no longer be guaranteed unique against the new one; and a failed attempt is
// not retried, so NextId keeps reporting the original error.
var ErrAlreadyInitialized = errors.New("idgenerator: global generator already initialized")

var (
	globalOnce sync.Once
	global     *idgen.DefaultIdGenerator
	globalErr  error
)

// Init builds the global generator with workerID, which must be in [0, 63].
// It returns ErrAlreadyInitialized if the generator was already built,
// including lazily by an earlier NextId, so call it from main before anything
// can generate an ID.
func Init(workerID uint16) error {
	if uint64(workerID) > maxWorkerID {
		return fmt.Errorf("idgenerator: invalid worker ID %d: must be in [0, %d]", workerID, maxWorkerID)
	}
	return initOnce(func() { install(workerID, nil) })
}

// InitFromEnv builds the global generator from WORKER_ID with the same rules
// NextId applies lazily (unset means 1). It returns the parse error for a
// malformed value, so main can fail at boot, and ErrAlreadyInitialized if the
// generator was already built.
func InitFromEnv() error {
	return initOnce(initFromEnv)
}

// NextId returns the next ID from the process-global generator. Its signature
// fits ent's DefaultFunc. If neither Init nor InitFromEnv ran, the first call
// builds the generator from WORKER_ID; when that value is malformed this and
// every later call panics, since no fallback worker ID can keep IDs unique.
func NextId() int64 {
	globalOnce.Do(initFromEnv)
	g := global
	if g == nil {
		// install leaves global nil only when it recorded globalErr.
		panic(globalErr)
	}
	return g.NewLong()
}

// initOnce runs f as the one-time initialization of the global generator and
// reports ErrAlreadyInitialized when that already happened.
func initOnce(f func()) error {
	ran := false
	globalOnce.Do(func() {
		ran = true
		f()
	})
	if !ran {
		return ErrAlreadyInitialized
	}
	return globalErr
}

func initFromEnv() {
	install(parseWorkerID(os.Getenv("WORKER_ID")))
}

// install records the outcome of initialization. An error is kept rather than
// replaced by a fallback worker ID, so every later NextId reports it.
func install(workerID uint16, err error) {
	if err != nil {
		globalErr = err
		return
	}
	global = newDefaultIdGenerator(workerID)
}

func newDefaultIdGenerator(workerID uint16) *idgen.DefaultIdGenerator {
	options := idgen.NewIdGeneratorOptions(workerID)
	options.BaseTime = baseTimeMillis
	return idgen.NewDefaultIdGenerator(options)
}

// NewIdGenerator returns an independent generator for workerID. Unique worker
// IDs are required across processes sharing a key space, and across
// generators within one process, including the global one. workerID must be
// in [0, 63]; unlike Init, NewIdGenerator panics instead of returning an error
// for a larger value. The returned function is safe for concurrent use.
func NewIdGenerator(workerID uint16) func() int64 {
	generator := newDefaultIdGenerator(workerID)
	return func() int64 {
		return generator.NewLong()
	}
}

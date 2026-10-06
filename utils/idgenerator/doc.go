// Package idgenerator produces process-unique, roughly time-ordered int64 IDs
// via yitter/idgenerator-go. It is not Twitter Snowflake (no datacenter bits,
// different layout).
//
// [NextId] is the process-global generator; its signature fits ent's
// DefaultFunc. Call [InitFromEnv] (or [Init]) early in main so a bad worker ID
// fails at boot rather than at the first insert. [NewIdGenerator] returns an
// independent generator for callers that manage worker IDs themselves.
//
// # Usage
//
//	import (
//		"log"
//
//		"github.com/go-sphere/sphere/utils/idgenerator"
//	)
//
//	func main() {
//		if err := idgenerator.InitFromEnv(); err != nil { // reads WORKER_ID
//			log.Fatal(err)
//		}
//		id := idgenerator.NextId()
//		_ = id
//	}
//
// # Worker IDs
//
// Every process sharing a key space needs a unique worker ID in [0, 63].
// WORKER_ID unset means 1, not 0, so StatefulSet pod-0 must set WORKER_ID=0
// explicitly. Importing the package has no side effects: the global generator
// is built on first use, either explicitly by Init or InitFromEnv, or lazily
// by the first NextId from WORKER_ID. A malformed WORKER_ID makes every NextId
// panic. Initialization happens once; later Init or InitFromEnv calls return
// [ErrAlreadyInitialized].
//
// IDs count ticks from a fixed epoch (2023-12-31T10:00:00Z), independent of
// time.Local. All functions are safe for concurrent use.
package idgenerator

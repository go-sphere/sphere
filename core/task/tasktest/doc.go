// Package tasktest provides reusable test helpers for task.Task: a [Fake]
// test double and [AssertLifecycleContract] for a subset of the lifecycle
// guarantees documented on the interface.
//
// AssertLifecycleContract checks that Stop is safe before Start, Stop is
// idempotent, and concurrent Stop does not panic or deadlock. It does not
// assert Stop-after-Start-returned, Stop-when-Start-failed, or that Stop
// unblocks Start without also cancelling ctx. The factory must return a fresh
// task that unblocks on Stop or ctx cancel; a Start that ignores both fails
// the helper after 5s.
//
// # Usage
//
//	import (
//		"testing"
//
//		"github.com/go-sphere/sphere/core/task"
//		"github.com/go-sphere/sphere/core/task/tasktest"
//	)
//
//	func TestWorkerLifecycle(t *testing.T) {
//		tasktest.AssertLifecycleContract(t, func() task.Task {
//			return NewWorker() // a fresh, not-yet-started task per call
//		})
//	}
//
// Use [NewFake] with a [Mode] to stand in for a dependency when testing a
// runner such as task.Group or task.Manager.
package tasktest

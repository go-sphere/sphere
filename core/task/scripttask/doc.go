// Package scripttask is a callback-backed task.Task test double that exposes
// Started/Stopped signals. It is not meant for production: with a nil onStart,
// Stop deliberately does not unblock Start, which the task.Task contract
// requires. Production code should use task.NewFunc.
//
// [NewScriptTask](id, onStart, onStop) returns a [ScriptTask]. Start runs
// onStart, or blocks on ctx.Done when onStart is nil. Stop runs onStop; it does
// not cancel Start's context. Direct Start+Stop with a nil onStart therefore
// deadlocks unless something else cancels ctx — put the task in a task.Group
// or task.Manager, which cancel the run context. onStop is not wrapped in
// Once: callers must make it idempotent. [ScriptTask.Started] and
// [ScriptTask.Stopped] close when Start or Stop is first entered, not when
// the hook returns.
//
// # Usage
//
//	import "github.com/go-sphere/sphere/core/task/scripttask"
//
//	worker := scripttask.NewScriptTask("worker", nil, nil) // Start blocks until ctx ends
//	runCtx, cancel := context.WithCancel(ctx)
//	done := make(chan error, 1)
//	go func() { done <- worker.Start(runCtx) }()
//	<-worker.Started()
//	cancel() // Stop alone would not unblock a nil onStart
//	err := <-done // context.Canceled
//	_ = worker.Stop(ctx)
//
// Inside a task.Group the group performs the cancel and Stop for you.
package scripttask

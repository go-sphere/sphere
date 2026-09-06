package task

import (
	"context"
)

// Func is a Task whose Start and Stop delegate to callbacks. Use it for
// one-off jobs instead of writing a dedicated type.
type Func struct {
	id      string
	onStart func(context.Context) error
	onStop  func(context.Context) error
}

// NewFunc constructs a Func. A nil onStart or onStop is a no-op that returns
// nil. Blocking, if needed, belongs in the callback: Start just runs onStart.
// Stop does not cancel Start's context; Group and Manager cancel the run
// context. onStop is not wrapped in Once: callers must make it idempotent.
func NewFunc(id string, onStart, onStop func(context.Context) error) *Func {
	return &Func{
		id:      id,
		onStart: onStart,
		onStop:  onStop,
	}
}

// Identifier returns the id passed to NewFunc.
func (f *Func) Identifier() string {
	return f.id
}

// Start runs onStart. A nil onStart returns nil.
func (f *Func) Start(ctx context.Context) error {
	if f.onStart != nil {
		return f.onStart(ctx)
	}
	return nil
}

// Stop runs onStop. Repeated calls run onStop again.
func (f *Func) Stop(ctx context.Context) error {
	if f.onStop != nil {
		return f.onStop(ctx)
	}
	return nil
}

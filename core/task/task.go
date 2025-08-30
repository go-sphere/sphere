package task

import (
	"context"
)

// Task defines the interface for lifecycle-managed components in the application.
// Tasks can be started and stopped with context support for graceful shutdown.
// This interface is commonly used for services, servers, workers, and other background operations.
type Task interface {
	// Identifier returns a unique identifier for this task.
	// This is used for logging and debugging purposes.
	Identifier() string

	// Start begins the task's operation with the given context.
	// The task should monitor the context for cancellation and stop gracefully when cancelled.
	// Returns an error if the task fails to start.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the task with the given context.
	// The context may have a deadline for shutdown completion.
	// Returns an error if the task fails to stop cleanly.
	Stop(ctx context.Context) error
}

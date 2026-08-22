package task

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-sphere/sphere/core/safe"
	"github.com/go-sphere/sphere/log"
)

func execute(ctx context.Context, name string, task Task, run func(ctx context.Context, task Task) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			safe.LogRecovered(task.Identifier(), r)
			err = fmt.Errorf("%s panic: %v", name, r)
		}
	}()
	err = run(ctx, task)
	if err != nil {
		if name != "" {
			err = fmt.Errorf("%s: %w", name, err)
		}
		// Cancellation provoked by this ctx is the runner tearing the task
		// down, not a failure. Logging it as Error makes a graceful shutdown
		// look like every member crashed. A Canceled that arrives while ctx
		// is still live is a real failure (see Group's wrapped-canceled guard)
		// and is still logged.
		if errors.Is(err, context.Canceled) && ctx != nil && ctx.Err() != nil {
			return err
		}
		logTaskError(task, name, err)
		return err
	}
	return
}

func logTaskError(task Task, name string, err error) {
	log.Error(
		fmt.Sprintf("%s error", name),
		log.String("task", task.Identifier()),
		log.Err(err),
	)
}

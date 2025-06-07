package task

import (
	"context"
	"fmt"

	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
)

func execute(ctx context.Context, name string, task Task, run func(ctx context.Context, task Task) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logTaskPanic(task, name, r)
			err = fmt.Errorf("%s panic: %v", name, r)
		}
	}()
	err = run(ctx, task)
	if err != nil {
		logTaskError(task, name, err)
		return
	}
	return
}

func logTaskPanic(task Task, name string, reason any) {
	log.Errorw(
		fmt.Sprintf("%s panic", name),
		logfields.String("task", task.Identifier()),
		logfields.Any("recover", reason),
	)
}

func logTaskError(task Task, name string, err error) {
	log.Errorw(
		fmt.Sprintf("%s error", name),
		logfields.String("task", task.Identifier()),
		logfields.Error(err),
	)
}

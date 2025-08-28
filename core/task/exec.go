package task

import (
	"context"
	"fmt"

	"github.com/go-sphere/sphere/log"
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
	log.Error(
		fmt.Sprintf("%s panic", name),
		log.String("task", task.Identifier()),
		log.Any("recover", reason),
	)
}

func logTaskError(task Task, name string, err error) {
	log.Error(
		fmt.Sprintf("%s error", name),
		log.String("task", task.Identifier()),
		log.Any("error", err),
	)
}

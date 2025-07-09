package testutils

import (
	"context"
	"errors"
	"time"

	"github.com/TBXark/sphere/core/task"
)

var _ task.Task = (*AutoClose)(nil)

type AutoClose struct{}

func NewAutoClose() *AutoClose {
	return &AutoClose{}
}

func (a AutoClose) Identifier() string {
	return "autoclose"
}

func (a AutoClose) Start(ctx context.Context) error {
	time.Sleep(3 * time.Second)
	return errors.New("simulated error for autoclose task")
}

func (a AutoClose) Stop(ctx context.Context) error {
	return nil
}

type AutoPanic struct{}

func NewAutoPanic() *AutoPanic {
	return &AutoPanic{}
}

func (a AutoPanic) Identifier() string {
	return "autopanic"
}

func (a AutoPanic) Start(ctx context.Context) error {
	time.Sleep(3 * time.Second)
	panic("simulated panic for autopanic task")
}

func (a AutoPanic) Stop(ctx context.Context) error {
	return nil
}

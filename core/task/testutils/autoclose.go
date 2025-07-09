package testutils

import (
	"context"
	"errors"
	"github.com/TBXark/sphere/core/task"
	"time"
)

var _ task.Task = (*AutoClose)(nil)

type AutoClose struct {
}

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

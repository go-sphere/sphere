//go:build longtest

package task_test

import (
	"context"
	"testing"
	"time"

	"github.com/TBXark/sphere/core/task"
	"github.com/TBXark/sphere/core/task/testtask"
	"github.com/stretchr/testify/assert"
)

func TestGroup_StartStop(t *testing.T) {
	server := testtask.NewServerExample()
	group := task.NewGroup(server)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(1 * time.Second)
		err := group.Stop(ctx)
		assert.NoError(t, err)
		cancel()
	}()

	err := group.Start(ctx)
	assert.ErrorIs(t, err, context.Canceled)
	assert.True(t, group.IsStarted())
	assert.True(t, group.IsStopped())
}

func TestGroup_StartWithError(t *testing.T) {
	autoClose := testtask.NewAutoClose()
	group := task.NewGroup(autoClose)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := group.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated error")
	assert.True(t, group.IsStarted())
}

func TestGroup_StartWithPanic(t *testing.T) {
	autoPanic := testtask.NewAutoPanic()
	group := task.NewGroup(autoPanic)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := group.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated panic")
	assert.True(t, group.IsStarted())
}

func TestGroup_DoubleStart(t *testing.T) {
	server := testtask.NewServerExample()
	group := task.NewGroup(server)
	ctx, cancel := context.WithCancel(context.Background())
	var err error
	go func() {
		time.Sleep(time.Second)
		cancel()
	}()
	go func() {
		err = group.Start(ctx)
		assert.ErrorIs(t, err, context.Canceled)
	}()
	time.Sleep(100 * time.Millisecond)
	err2 := group.Start(ctx)
	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "already started")
	<-ctx.Done()
}

func TestGroup_StopWithoutStart(t *testing.T) {
	group := task.NewGroup()
	err := group.Stop(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not started")
}

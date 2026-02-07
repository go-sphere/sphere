package task

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-sphere/sphere/core/task/testtask"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroup_StartStop(t *testing.T) {
	tests := []struct {
		name           string
		setupTasks     func() []Task
		stopDelay      time.Duration
		expectedErr    error
		checkStarted   bool
		checkStopped   bool
		additionalTest func(t *testing.T, tasks []Task)
	}{
		{
			name: "single success task - normal stop",
			setupTasks: func() []Task {
				return []Task{testtask.NewSuccessTask("task1")}
			},
			stopDelay:    1 * time.Second,
			expectedErr:  context.Canceled,
			checkStarted: true,
			checkStopped: true,
		},
		{
			name: "multiple success tasks - normal stop",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewSuccessTask("task1"),
					testtask.NewSuccessTask("task2"),
					testtask.NewSuccessTask("task3"),
				}
			},
			stopDelay:    500 * time.Millisecond,
			expectedErr:  context.Canceled,
			checkStarted: true,
			checkStopped: true,
			additionalTest: func(t *testing.T, tasks []Task) {
				for _, tk := range tasks {
					st := tk.(*testtask.SuccessTask)
					assert.True(t, st.IsStarted(), "task %s should be started", st.Identifier())
					assert.True(t, st.IsStopped(), "task %s should be stopped", st.Identifier())
				}
			},
		},
		{
			name: "server example task",
			setupTasks: func() []Task {
				return []Task{testtask.NewServerExample()}
			},
			stopDelay:    1 * time.Second,
			expectedErr:  context.Canceled, // Server returns ErrServerClosed, but final error is context.Canceled
			checkStarted: true,
			checkStopped: true,
		},
		{
			name: "slow start tasks",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewSlowStartTask("slow1", 200*time.Millisecond),
					testtask.NewSlowStartTask("slow2", 300*time.Millisecond),
				}
			},
			stopDelay:    1 * time.Second,
			expectedErr:  context.Canceled,
			checkStarted: true,
			checkStopped: true,
			additionalTest: func(t *testing.T, tasks []Task) {
				for _, tk := range tasks {
					st := tk.(*testtask.SlowStartTask)
					assert.True(t, st.IsStarted(), "task %s should be started", st.Identifier())
					assert.True(t, st.IsStopped(), "task %s should be stopped", st.Identifier())
				}
			},
		},
		{
			name: "context aware tasks",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewContextAwareTask("ctx1"),
					testtask.NewContextAwareTask("ctx2"),
				}
			},
			stopDelay:    500 * time.Millisecond,
			expectedErr:  context.Canceled,
			checkStarted: true,
			checkStopped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := tt.setupTasks()
			group := NewGroup(tasks...)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go func() {
				time.Sleep(tt.stopDelay)
				err := group.Stop(ctx)
				assert.NoError(t, err)
				cancel()
			}()

			err := group.Start(ctx)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			if tt.checkStarted {
				assert.True(t, group.IsStarted())
			}
			if tt.checkStopped {
				assert.True(t, group.IsStopped())
			}

			if tt.additionalTest != nil {
				tt.additionalTest(t, tasks)
			}
		})
	}
}

func TestGroup_StartWithError(t *testing.T) {
	tests := []struct {
		name               string
		setupTasks         func() []Task
		expectedErrMsg     string
		timeout            time.Duration
		checkOthersStopped func(t *testing.T, tasks []Task)
	}{
		{
			name: "single task fails immediately",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewFailingStartTask("fail1", errors.New("startup failed")),
				}
			},
			expectedErrMsg: "startup failed",
			timeout:        2 * time.Second,
		},
		{
			name: "one task fails, others should stop",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewSuccessTask("success1"),
					testtask.NewFailingStartTask("fail1", errors.New("task failed")),
					testtask.NewSuccessTask("success2"),
				}
			},
			expectedErrMsg: "task failed",
			timeout:        2 * time.Second,
			checkOthersStopped: func(t *testing.T, tasks []Task) {
				// Check that success tasks were stopped
				st1 := tasks[0].(*testtask.SuccessTask)
				st2 := tasks[2].(*testtask.SuccessTask)
				assert.True(t, st1.IsStopped(), "success1 should be stopped")
				assert.True(t, st2.IsStopped(), "success2 should be stopped")
			},
		},
		{
			name: "autoclose task error after delay",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewAutoCloseWithDelay(500 * time.Millisecond),
				}
			},
			expectedErrMsg: "simulated error",
			timeout:        5 * time.Second,
		},
		{
			name: "mixed tasks with one failure",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewSuccessTask("success1"),
					testtask.NewSlowStartTask("slow1", 200*time.Millisecond),
					testtask.NewFailingStartTask("fail1", errors.New("critical error")),
					testtask.NewContextAwareTask("ctx1"),
				}
			},
			expectedErrMsg: "critical error",
			timeout:        3 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := tt.setupTasks()
			group := NewGroup(tasks...)

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			err := group.Start(ctx)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrMsg)
			assert.True(t, group.IsStarted())

			if tt.checkOthersStopped != nil {
				tt.checkOthersStopped(t, tasks)
			}
		})
	}
}

func TestGroup_StartWithPanic(t *testing.T) {
	tests := []struct {
		name           string
		setupTasks     func() []Task
		expectedErrMsg string
		timeout        time.Duration
	}{
		{
			name: "single panic task",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewAutoPanicWithDelay(500 * time.Millisecond),
				}
			},
			expectedErrMsg: "simulated panic",
			timeout:        5 * time.Second,
		},
		{
			name: "panic with other tasks",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewSuccessTask("success1"),
					testtask.NewAutoPanicWithDelay(300 * time.Millisecond),
					testtask.NewSuccessTask("success2"),
				}
			},
			expectedErrMsg: "simulated panic",
			timeout:        5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := tt.setupTasks()
			group := NewGroup(tasks...)

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			err := group.Start(ctx)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrMsg)
			assert.True(t, group.IsStarted())
		})
	}
}

func TestGroup_LifecycleStates(t *testing.T) {
	tests := []struct {
		name        string
		operation   func(t *testing.T, group *Group, ctx context.Context) error
		expectedErr string
	}{
		{
			name: "double start",
			operation: func(t *testing.T, group *Group, ctx context.Context) error {
				go func() {
					time.Sleep(100 * time.Millisecond)
					_ = group.Stop(ctx)
				}()
				err1 := group.Start(ctx)
				if err1 != nil && !errors.Is(err1, context.Canceled) {
					return err1
				}
				time.Sleep(200 * time.Millisecond) // Ensure stopped
				return group.Start(ctx)
			},
			expectedErr: "already stopped", // After first Start completes and Stop is called
		},
		{
			name: "stop without start",
			operation: func(t *testing.T, group *Group, ctx context.Context) error {
				return group.Stop(ctx)
			},
			expectedErr: "not started",
		},
		{
			name: "double stop",
			operation: func(t *testing.T, group *Group, ctx context.Context) error {
				go func() {
					err := group.Start(ctx)
					assert.ErrorIs(t, err, context.Canceled)
				}()
				time.Sleep(100 * time.Millisecond)
				err1 := group.Stop(ctx)
				assert.NoError(t, err1)
				time.Sleep(100 * time.Millisecond)
				return group.Stop(ctx)
			},
			expectedErr: "", // Should return nil
		},
		{
			name: "start after stop",
			operation: func(t *testing.T, group *Group, ctx context.Context) error {
				done := make(chan error, 1)
				go func() {
					done <- group.Start(ctx)
				}()
				time.Sleep(100 * time.Millisecond)
				err := group.Stop(ctx)
				assert.NoError(t, err)
				<-done // Wait for first Start to complete
				time.Sleep(100 * time.Millisecond)
				return group.Start(ctx)
			},
			expectedErr: "already stopped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := testtask.NewServerExample()
			group := NewGroup(server)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := tt.operation(t, group, ctx)
			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGroup_StopBehavior(t *testing.T) {
	tests := []struct {
		name         string
		setupTasks   func() []Task
		timeout      time.Duration
		checkStopped func(t *testing.T, tasks []Task)
	}{
		{
			name: "tasks with failing stop",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewSuccessTask("success1"),
					testtask.NewFailingStopTask("failstop1", errors.New("stop failed")),
					testtask.NewSuccessTask("success2"),
				}
			},
			timeout: 3 * time.Second,
			checkStopped: func(t *testing.T, tasks []Task) {
				fst := tasks[1].(*testtask.FailingStopTask)
				assert.True(t, fst.IsStopped(), "failing stop task should have stop called")
			},
		},
		{
			name: "slow stop tasks",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewSlowStopTask("slow1", 200*time.Millisecond),
					testtask.NewSlowStopTask("slow2", 300*time.Millisecond),
					testtask.NewSuccessTask("fast"),
				}
			},
			timeout: 3 * time.Second,
			checkStopped: func(t *testing.T, tasks []Task) {
				slow1 := tasks[0].(*testtask.SlowStopTask)
				slow2 := tasks[1].(*testtask.SlowStopTask)
				fast := tasks[2].(*testtask.SuccessTask)
				assert.True(t, slow1.IsStopped())
				assert.True(t, slow2.IsStopped())
				assert.True(t, fast.IsStopped())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := tt.setupTasks()
			group := NewGroup(tasks...)

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			go func() {
				time.Sleep(500 * time.Millisecond)
				_ = group.Stop(ctx)
				cancel()
			}()

			err := group.Start(ctx)
			assert.ErrorIs(t, err, context.Canceled)
			assert.True(t, group.IsStopped())

			if tt.checkStopped != nil {
				tt.checkStopped(t, tasks)
			}
		})
	}
}

func TestGroup_ContextCancellation(t *testing.T) {
	tests := []struct {
		name        string
		setupTasks  func() []Task
		cancelDelay time.Duration
		expectedErr error
	}{
		{
			name: "parent context cancelled",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewSuccessTask("task1"),
					testtask.NewSuccessTask("task2"),
				}
			},
			cancelDelay: 500 * time.Millisecond,
			expectedErr: context.Canceled,
		},
		{
			name: "parent context cancelled with slow tasks",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewSlowStartTask("slow1", 200*time.Millisecond),
					testtask.NewContextAwareTask("ctx1"),
				}
			},
			cancelDelay: 1 * time.Second,
			expectedErr: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := tt.setupTasks()
			group := NewGroup(tasks...)

			ctx, cancel := context.WithCancel(context.Background())

			go func() {
				time.Sleep(tt.cancelDelay)
				cancel()
			}()

			err := group.Start(ctx)
			assert.ErrorIs(t, err, tt.expectedErr)
			assert.True(t, group.IsStarted())
			assert.True(t, group.IsStopped())
		})
	}
}

func TestGroup_EmptyGroup(t *testing.T) {
	group := NewGroup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = group.Stop(ctx)
		cancel()
	}()

	err := group.Start(ctx)
	assert.ErrorIs(t, err, context.Canceled)
	assert.True(t, group.IsStarted())
	assert.True(t, group.IsStopped())
}

func TestGroup_NestedGroups(t *testing.T) {
	innerGroup1 := NewGroup(
		testtask.NewSuccessTask("inner1-task1"),
		testtask.NewSuccessTask("inner1-task2"),
	)

	innerGroup2 := NewGroup(
		testtask.NewSuccessTask("inner2-task1"),
		testtask.NewContextAwareTask("inner2-task2"),
	)

	outerGroup := NewGroup(innerGroup1, innerGroup2)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(1 * time.Second)
		err := outerGroup.Stop(ctx)
		assert.NoError(t, err)
		cancel()
	}()

	err := outerGroup.Start(ctx)
	assert.ErrorIs(t, err, context.Canceled)
	assert.True(t, outerGroup.IsStarted())
	assert.True(t, outerGroup.IsStopped())
	assert.True(t, innerGroup1.IsStarted())
	assert.True(t, innerGroup1.IsStopped())
	assert.True(t, innerGroup2.IsStarted())
	assert.True(t, innerGroup2.IsStopped())
}

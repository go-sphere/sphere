package task

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-sphere/sphere/core/task/testtask"
)

func TestGroup_StartStop(t *testing.T) {
	tests := []struct {
		name       string
		setupTasks func() []Task
		stopDelay  time.Duration
		verify     func(t *testing.T, tasks []Task)
	}{
		{
			name: "single success task",
			setupTasks: func() []Task {
				return []Task{testtask.NewSuccessTask("task1")}
			},
			stopDelay: 300 * time.Millisecond,
			verify: func(t *testing.T, tasks []Task) {
				t.Helper()
				task1, ok := tasks[0].(*testtask.SuccessTask)
				if !ok {
					t.Fatalf("tasks[0] type = %T, want *testtask.SuccessTask", tasks[0])
				}
				if !task1.IsStarted() {
					t.Fatal("task1 should be started")
				}
				if !task1.IsStopped() {
					t.Fatal("task1 should be stopped")
				}
			},
		},
		{
			name: "multiple success tasks",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewSuccessTask("task1"),
					testtask.NewSuccessTask("task2"),
					testtask.NewSuccessTask("task3"),
				}
			},
			stopDelay: 300 * time.Millisecond,
			verify: func(t *testing.T, tasks []Task) {
				t.Helper()
				for i, tt := range tasks {
					taskN, ok := tt.(*testtask.SuccessTask)
					if !ok {
						t.Fatalf("tasks[%d] type = %T, want *testtask.SuccessTask", i, tt)
					}
					if !taskN.IsStarted() {
						t.Fatalf("%s should be started", taskN.Identifier())
					}
					if !taskN.IsStopped() {
						t.Fatalf("%s should be stopped", taskN.Identifier())
					}
				}
			},
		},
		{
			name: "server example task",
			setupTasks: func() []Task {
				return []Task{testtask.NewServerExample()}
			},
			stopDelay: 600 * time.Millisecond,
		},
		{
			name: "slow start tasks",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewSlowStartTask("slow1", 150*time.Millisecond),
					testtask.NewSlowStartTask("slow2", 250*time.Millisecond),
				}
			},
			stopDelay: 700 * time.Millisecond,
			verify: func(t *testing.T, tasks []Task) {
				t.Helper()
				for i, tt := range tasks {
					taskN, ok := tt.(*testtask.SlowStartTask)
					if !ok {
						t.Fatalf("tasks[%d] type = %T, want *testtask.SlowStartTask", i, tt)
					}
					if !taskN.IsStarted() {
						t.Fatalf("%s should be started", taskN.Identifier())
					}
					if !taskN.IsStopped() {
						t.Fatalf("%s should be stopped", taskN.Identifier())
					}
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
			stopDelay: 300 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tasks := tt.setupTasks()
			group := NewGroup(tasks...)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			stopErrCh := make(chan error, 1)
			go func() {
				time.Sleep(tt.stopDelay)
				stopErrCh <- group.Stop(context.Background())
			}()

			err := group.Start(ctx)
			mustErrorIs(t, err, context.Canceled)
			mustNoError(t, waitErr(stopErrCh, 2*time.Second), "Stop should succeed")

			if !group.IsStarted() {
				t.Fatal("group should be started")
			}
			if !group.IsStopped() {
				t.Fatal("group should be stopped")
			}

			if tt.verify != nil {
				tt.verify(t, tasks)
			}
		})
	}
}

func TestGroup_StartWithError(t *testing.T) {
	tests := []struct {
		name       string
		setupTasks func() []Task
		wantSubstr string
		verify     func(t *testing.T, tasks []Task)
	}{
		{
			name: "single task fails immediately",
			setupTasks: func() []Task {
				return []Task{testtask.NewFailingStartTask("fail1", errors.New("startup failed"))}
			},
			wantSubstr: "startup failed",
		},
		{
			name: "one task fails and other tasks are stopped",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewSuccessTask("success1"),
					testtask.NewFailingStartTask("fail1", errors.New("task failed")),
					testtask.NewSuccessTask("success2"),
				}
			},
			wantSubstr: "task failed",
			verify: func(t *testing.T, tasks []Task) {
				t.Helper()
				s1, ok := tasks[0].(*testtask.SuccessTask)
				if !ok {
					t.Fatalf("tasks[0] type = %T, want *testtask.SuccessTask", tasks[0])
				}
				s2, ok := tasks[2].(*testtask.SuccessTask)
				if !ok {
					t.Fatalf("tasks[2] type = %T, want *testtask.SuccessTask", tasks[2])
				}
				if !s1.IsStopped() {
					t.Fatal("success1 should be stopped")
				}
				if !s2.IsStopped() {
					t.Fatal("success2 should be stopped")
				}
			},
		},
		{
			name: "autoclose task fails after delay",
			setupTasks: func() []Task {
				return []Task{testtask.NewAutoCloseWithDelay(250 * time.Millisecond)}
			},
			wantSubstr: "simulated error",
		},
		{
			name: "mixed tasks with one failing task",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewSuccessTask("success1"),
					testtask.NewSlowStartTask("slow1", 100*time.Millisecond),
					testtask.NewFailingStartTask("fail1", errors.New("critical error")),
					testtask.NewContextAwareTask("ctx1"),
				}
			},
			wantSubstr: "critical error",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tasks := tt.setupTasks()
			group := NewGroup(tasks...)

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			err := group.Start(ctx)
			mustErrorContains(t, err, tt.wantSubstr)

			if !group.IsStarted() {
				t.Fatal("group should be started")
			}

			if tt.verify != nil {
				tt.verify(t, tasks)
			}
		})
	}
}

func TestGroup_StartWithPanic(t *testing.T) {
	tests := []struct {
		name       string
		setupTasks func() []Task
		wantSubstr string
	}{
		{
			name: "single panic task",
			setupTasks: func() []Task {
				return []Task{testtask.NewAutoPanicWithDelay(200 * time.Millisecond)}
			},
			wantSubstr: "simulated panic",
		},
		{
			name: "panic task with other running tasks",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewSuccessTask("success1"),
					testtask.NewAutoPanicWithDelay(200 * time.Millisecond),
					testtask.NewSuccessTask("success2"),
				}
			},
			wantSubstr: "simulated panic",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			group := NewGroup(tt.setupTasks()...)

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			err := group.Start(ctx)
			mustErrorContains(t, err, tt.wantSubstr)
			if !group.IsStarted() {
				t.Fatal("group should be started")
			}
		})
	}
}

func TestGroup_LifecycleStates(t *testing.T) {
	t.Run("stop without start", func(t *testing.T) {
		group := NewGroup(testtask.NewSuccessTask("task1"))
		err := group.Stop(context.Background())
		mustErrorContains(t, err, "not started")
		if !group.IsStopped() {
			t.Fatal("group should be marked as stopped")
		}
	})

	t.Run("double stop returns nil", func(t *testing.T) {
		group := NewGroup(testtask.NewSuccessTask("task1"))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		startErrCh := make(chan error, 1)
		go func() {
			startErrCh <- group.Start(ctx)
		}()

		waitFor(t, time.Second, group.IsStarted, "group did not start")
		mustNoError(t, group.Stop(context.Background()), "first Stop should succeed")
		mustNoError(t, group.Stop(context.Background()), "second Stop should be idempotent")
		mustErrorIs(t, waitErr(startErrCh, 2*time.Second), context.Canceled)
	})

	t.Run("double start while running", func(t *testing.T) {
		group := NewGroup(testtask.NewSuccessTask("task1"))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		startErrCh := make(chan error, 1)
		go func() {
			startErrCh <- group.Start(ctx)
		}()

		waitFor(t, time.Second, group.IsStarted, "group did not start")
		err := group.Start(context.Background())
		mustErrorContains(t, err, "already started")

		mustNoError(t, group.Stop(context.Background()), "Stop should succeed")
		mustErrorIs(t, waitErr(startErrCh, 2*time.Second), context.Canceled)
	})

	t.Run("start after stop", func(t *testing.T) {
		group := NewGroup(testtask.NewSuccessTask("task1"))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		startErrCh := make(chan error, 1)
		go func() {
			startErrCh <- group.Start(ctx)
		}()

		waitFor(t, time.Second, group.IsStarted, "group did not start")
		mustNoError(t, group.Stop(context.Background()), "Stop should succeed")
		mustErrorIs(t, waitErr(startErrCh, 2*time.Second), context.Canceled)

		err := group.Start(context.Background())
		mustErrorContains(t, err, "already stopped")
	})
}

func TestGroup_StopBehavior(t *testing.T) {
	tests := []struct {
		name       string
		setupTasks func() []Task
		verify     func(t *testing.T, tasks []Task)
	}{
		{
			name: "failing stop task still receives Stop",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewSuccessTask("success1"),
					testtask.NewFailingStopTask("failstop1", errors.New("stop failed")),
					testtask.NewSuccessTask("success2"),
				}
			},
			verify: func(t *testing.T, tasks []Task) {
				t.Helper()
				failing, ok := tasks[1].(*testtask.FailingStopTask)
				if !ok {
					t.Fatalf("tasks[1] type = %T, want *testtask.FailingStopTask", tasks[1])
				}
				if !failing.IsStopped() {
					t.Fatal("failing stop task should have Stop called")
				}
			},
		},
		{
			name: "slow stop tasks eventually stop",
			setupTasks: func() []Task {
				return []Task{
					testtask.NewSlowStopTask("slow1", 150*time.Millisecond),
					testtask.NewSlowStopTask("slow2", 250*time.Millisecond),
					testtask.NewSuccessTask("fast"),
				}
			},
			verify: func(t *testing.T, tasks []Task) {
				t.Helper()
				slow1, ok := tasks[0].(*testtask.SlowStopTask)
				if !ok {
					t.Fatalf("tasks[0] type = %T, want *testtask.SlowStopTask", tasks[0])
				}
				slow2, ok := tasks[1].(*testtask.SlowStopTask)
				if !ok {
					t.Fatalf("tasks[1] type = %T, want *testtask.SlowStopTask", tasks[1])
				}
				fast, ok := tasks[2].(*testtask.SuccessTask)
				if !ok {
					t.Fatalf("tasks[2] type = %T, want *testtask.SuccessTask", tasks[2])
				}
				if !slow1.IsStopped() || !slow2.IsStopped() || !fast.IsStopped() {
					t.Fatal("all tasks should be stopped")
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tasks := tt.setupTasks()
			group := NewGroup(tasks...)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			stopErrCh := make(chan error, 1)
			go func() {
				time.Sleep(350 * time.Millisecond)
				stopErrCh <- group.Stop(context.Background())
			}()

			err := group.Start(ctx)
			mustErrorIs(t, err, context.Canceled)
			mustNoError(t, waitErr(stopErrCh, 2*time.Second), "Stop should succeed")

			if !group.IsStopped() {
				t.Fatal("group should be stopped")
			}

			if tt.verify != nil {
				tt.verify(t, tasks)
			}
		})
	}
}

func TestGroup_ContextCancellation(t *testing.T) {
	tests := []struct {
		name       string
		setupTasks func() []Task
		cancelIn   time.Duration
	}{
		{
			name: "cancel parent context with success tasks",
			setupTasks: func() []Task {
				return []Task{testtask.NewSuccessTask("task1"), testtask.NewSuccessTask("task2")}
			},
			cancelIn: 300 * time.Millisecond,
		},
		{
			name: "cancel parent context with mixed tasks",
			setupTasks: func() []Task {
				return []Task{testtask.NewSlowStartTask("slow1", 100*time.Millisecond), testtask.NewContextAwareTask("ctx1")}
			},
			cancelIn: 450 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			group := NewGroup(tt.setupTasks()...)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go func() {
				time.Sleep(tt.cancelIn)
				cancel()
			}()

			err := group.Start(ctx)
			mustErrorIs(t, err, context.Canceled)
			if !group.IsStarted() {
				t.Fatal("group should be started")
			}
			if !group.IsStopped() {
				t.Fatal("group should be stopped")
			}
		})
	}
}

func TestGroup_EmptyGroup(t *testing.T) {
	group := NewGroup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = group.Stop(context.Background())
	}()

	err := group.Start(ctx)
	mustErrorIs(t, err, context.Canceled)
	if !group.IsStarted() {
		t.Fatal("group should be started")
	}
	if !group.IsStopped() {
		t.Fatal("group should be stopped")
	}
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
	defer cancel()

	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = outerGroup.Stop(context.Background())
	}()

	err := outerGroup.Start(ctx)
	mustErrorIs(t, err, context.Canceled)

	if !outerGroup.IsStarted() || !outerGroup.IsStopped() {
		t.Fatal("outer group should be started and stopped")
	}
	if !innerGroup1.IsStarted() || !innerGroup1.IsStopped() {
		t.Fatal("innerGroup1 should be started and stopped")
	}
	if !innerGroup2.IsStarted() || !innerGroup2.IsStopped() {
		t.Fatal("innerGroup2 should be started and stopped")
	}
}

func mustNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

func mustErrorIs(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("error = %v, want errors.Is(_, %v)", err, target)
	}
}

func mustErrorContains(t *testing.T, err error, sub string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", sub)
	}
	if !strings.Contains(err.Error(), sub) {
		t.Fatalf("error = %q, want substring %q", err.Error(), sub)
	}
}

func waitErr(ch <-chan error, timeout time.Duration) error {
	select {
	case err := <-ch:
		return err
	case <-time.After(timeout):
		return errors.New("timeout waiting for goroutine result")
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool, failMsg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(failMsg)
}

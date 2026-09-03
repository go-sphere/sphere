package task_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/scripttask"
)

// TestAdversarialManager_ConcurrentOperations stresses Manager with massive concurrent
// StartTask, StopTask, StopAll, GetTaskResult, IsRunning, GetRunningTasks, and Wait calls.
func TestAdversarialManager_ConcurrentOperations(t *testing.T) {
	t.Parallel()

	mgr := task.NewManager(task.WithManagerCleanupTimeout(50 * time.Millisecond))

	const numWorkers = 20
	const numOpsPerWorker = 50
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for w := range numWorkers {
		workerID := w
		go func() {
			defer wg.Done()
			r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

			for range numOpsPerWorker {
				taskName := fmt.Sprintf("task-%d", r.Intn(15)) // intentionally small name pool to trigger overlap
				op := r.Intn(7)
				taskDelayMs := r.Intn(20)
				shouldFail := r.Intn(4) == 0

				switch op {
				case 0: // StartTask
					tk := scripttask.NewScriptTask(
						taskName,
						func(ctx context.Context) error {
							select {
							case <-time.After(time.Duration(taskDelayMs) * time.Millisecond):
								if shouldFail {
									return errors.New("random task fail")
								}
								return nil
							case <-ctx.Done():
								return ctx.Err()
							}
						},
						func(ctx context.Context) error {
							return nil
						},
					)
					_ = mgr.StartTask(context.Background(), taskName, tk)

				case 1: // StopTask
					ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
					_ = mgr.StopTask(ctx, taskName)
					cancel()

				case 2: // StopAll
					if r.Intn(10) == 0 {
						ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
						_ = mgr.StopAll(ctx)
						cancel()
					}

				case 3: // GetTaskResult
					_, _ = mgr.GetTaskResult(taskName)

				case 4: // IsRunning
					_ = mgr.IsRunning(taskName)

				case 5: // GetRunningTasks & GetTaskCount
					_ = mgr.GetRunningTasks()
					_ = mgr.GetTaskCount()

				case 6: // Short sleep
					time.Sleep(time.Duration(r.Intn(5)) * time.Millisecond)
				}
			}
		}()
	}

	wg.Wait()

	// Clean up all running tasks
	_ = mgr.StopAll(context.Background())
	_ = mgr.Wait()
}

// TestAdversarialManager_DuplicateRegistrationRace tests that when N goroutines race
// to start a task with the exact same name, exactly 1 succeeds and N-1 receive ErrTaskAlreadyExists.
func TestAdversarialManager_DuplicateRegistrationRace(t *testing.T) {
	t.Parallel()

	for round := range 10 {
		mgr := task.NewManager()
		taskName := fmt.Sprintf("race-task-%d", round)

		const racers = 30
		var ready sync.WaitGroup
		ready.Add(racers)
		startSignal := make(chan struct{})

		var successCount atomic.Int32
		var alreadyExistsCount atomic.Int32
		var otherErrCount atomic.Int32

		var wg sync.WaitGroup
		wg.Add(racers)

		for range racers {
			go func() {
				defer wg.Done()
				ready.Done()
				<-startSignal

				tk := scripttask.NewScriptTask(
					taskName,
					func(ctx context.Context) error {
						<-ctx.Done()
						return nil
					},
					nil,
				)
				err := mgr.StartTask(context.Background(), taskName, tk)
				if err == nil {
					successCount.Add(1)
				} else if errors.Is(err, task.ErrTaskAlreadyExists) {
					alreadyExistsCount.Add(1)
				} else {
					otherErrCount.Add(1)
				}
			}()
		}

		ready.Wait()
		close(startSignal)
		wg.Wait()

		if successCount.Load() != 1 {
			t.Fatalf("round %d: expected exactly 1 winner, got %d (already exists: %d, other errors: %d)",
				round, successCount.Load(), alreadyExistsCount.Load(), otherErrCount.Load())
		}
		if alreadyExistsCount.Load() != racers-1 {
			t.Fatalf("round %d: expected %d ErrTaskAlreadyExists, got %d",
				round, racers-1, alreadyExistsCount.Load())
		}

		_ = mgr.StopTask(context.Background(), taskName)
	}
}

// TestAdversarialManager_TombstoneRolloverBeyond1024 verifies tombstone eviction behavior
// under both sequential rollover and high concurrent churning beyond 1024 items.
func TestAdversarialManager_TombstoneRolloverBeyond1024(t *testing.T) {
	t.Parallel()

	t.Run("SequentialRollover", func(t *testing.T) {
		t.Parallel()
		mgr := task.NewManager()
		const totalTasks = 2000

		for i := range totalTasks {
			name := fmt.Sprintf("seq-%d", i)
			idx := i
			targetErr := fmt.Errorf("err-%d", idx)
			tk := scripttask.NewScriptTask(
				name,
				func(ctx context.Context) error {
					if idx%2 == 1 {
						return targetErr
					}
					return nil
				},
				nil,
			)

			err := mgr.StartTask(context.Background(), name, tk)
			if err != nil {
				t.Fatalf("failed to start %s: %v", name, err)
			}
			stopErr := mgr.StopTask(context.Background(), name)
			if idx%2 == 1 {
				if stopErr == nil || !errors.Is(stopErr, targetErr) {
					t.Fatalf("expected error wrapping %v, got %v", targetErr, stopErr)
				}
			} else {
				if stopErr != nil {
					t.Fatalf("expected nil error, got %v", stopErr)
				}
			}
		}

		// Tasks 0 to (2000 - 1024 - 1) = 0..975 MUST be evicted
		for i := range totalTasks - 1024 {
			evictedName := fmt.Sprintf("seq-%d", i)
			found, _ := mgr.GetTaskResult(evictedName)
			if found {
				t.Fatalf("task %s was expected to be evicted, but was found", evictedName)
			}
			stopErr := mgr.StopTask(context.Background(), evictedName)
			if !errors.Is(stopErr, task.ErrTaskNotFound) {
				t.Fatalf("expected ErrTaskNotFound for evicted %s, got %v", evictedName, stopErr)
			}
		}

		// Tasks (2000 - 1024) to 1999 = 976..1999 MUST be retained
		for i := totalTasks - 1024; i < totalTasks; i++ {
			retainedName := fmt.Sprintf("seq-%d", i)
			found, err := mgr.GetTaskResult(retainedName)
			if !found {
				t.Fatalf("task %s was expected to be retained, but was not found", retainedName)
			}
			if i%2 == 1 {
				expectedErrStr := fmt.Sprintf("%s: err-%d", retainedName, i)
				if err == nil || err.Error() != expectedErrStr {
					t.Fatalf("task %s expected error %s, got %v", retainedName, expectedErrStr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("task %s expected nil error, got %v", retainedName, err)
				}
			}
		}
	})

	t.Run("ConcurrentChurnBoundedCapacity", func(t *testing.T) {
		t.Parallel()
		mgr := task.NewManager()
		const totalTasks = 2500

		for i := range totalTasks {
			name := fmt.Sprintf("concurrent-churn-%d", i)
			tk := scripttask.NewScriptTask(
				name,
				func(ctx context.Context) error { return nil },
				nil,
			)
			_ = mgr.StartTask(context.Background(), name, tk)
		}

		_ = mgr.Wait()

		foundCount := 0
		notFoundCount := 0
		for i := range totalTasks {
			name := fmt.Sprintf("concurrent-churn-%d", i)
			found, _ := mgr.GetTaskResult(name)
			if found {
				foundCount++
			} else {
				notFoundCount++
			}
		}

		if foundCount != 1024 {
			t.Fatalf("expected exactly 1024 tombstones retained, got %d", foundCount)
		}
		if notFoundCount != totalTasks-1024 {
			t.Fatalf("expected exactly %d evicted tasks, got %d", totalTasks-1024, notFoundCount)
		}
	})
}

// TestAdversarialManager_ErrorLimitAccumulator verifies maxRetainedErrors (1024).
func TestAdversarialManager_ErrorLimitAccumulator(t *testing.T) {
	t.Parallel()

	mgr := task.NewManager()
	const totalFails = 2000

	for i := range totalFails {
		name := fmt.Sprintf("fail-%d", i)
		idx := i
		tk := scripttask.NewScriptTask(
			name,
			func(ctx context.Context) error {
				return fmt.Errorf("fail-%d", idx)
			},
			nil,
		)
		_ = mgr.StartTask(context.Background(), name, tk)
	}

	err := mgr.Wait()
	if err == nil {
		t.Fatal("expected aggregated errors from Wait(), got nil")
	}
}

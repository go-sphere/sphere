package task_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/scripttask"
)

// TestManagerDuplicateRegistrationRace tests that when N goroutines race
// to start a task with the exact same name, exactly 1 succeeds and N-1 receive ErrTaskAlreadyExists.
func TestManagerDuplicateRegistrationRace(t *testing.T) {
	t.Parallel()

	for round := range 3 {
		mgr := task.NewManager()
		taskName := fmt.Sprintf("race-task-%d", round)

		const racers = 16
		var ready sync.WaitGroup
		ready.Add(racers)
		startSignal := make(chan struct{})

		var successCount atomic.Int32
		var alreadyExistsCount atomic.Int32
		var otherErrCount atomic.Int32

		var wg sync.WaitGroup

		for range racers {
			wg.Go(func() {
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
				err := mgr.StartTask(t.Context(), taskName, tk)
				if err == nil {
					successCount.Add(1)
				} else if errors.Is(err, task.ErrTaskAlreadyExists) {
					alreadyExistsCount.Add(1)
				} else {
					otherErrCount.Add(1)
				}
			})
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

		if err := mgr.StopTask(t.Context(), taskName); err != nil {
			t.Fatalf("round %d stop winner: %v", round, err)
		}
	}
}

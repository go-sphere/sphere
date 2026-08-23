package task

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-sphere/sphere/core/task/multierr"
	"github.com/go-sphere/sphere/log"
)

// Start runs the group's members and does not return until every task whose
// Start was invoked has also been Stop'd.
//
// One-shot jobs: every Start returns on its own, then the group stops those
// tasks (cleanup) and Start returns.
//
// Long-running servers: the last stage blocks inside Start until Stop or the
// parent context is cancelled; the group then stops started stages in reverse
// order and Start returns.
func (g *Group) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateTasks(g.tasks); err != nil {
		return err
	}
	stopReqCh, doneCh, err := g.beginLifecycle()
	if err != nil {
		return err
	}

	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	r := &groupRun{
		g:         g,
		parent:    ctx,
		runCtx:    runCtx,
		runCancel: runCancel,
		stopReqCh: stopReqCh,
		stopDone:  make(chan struct{}),
	}
	if g.waves == nil {
		r.stages = [][]Task{g.tasks}
	} else {
		r.stages = g.waves
	}

	finalErr := r.loop()
	g.finishLifecycle(doneCh, finalErr)
	return finalErr
}

func validateTasks(tasks []Task) error {
	for idx, t := range tasks {
		if t == nil {
			return fmt.Errorf("task at index %d is nil", idx)
		}
	}
	return nil
}

func (g *Group) beginLifecycle() (chan shutdownReason, chan struct{}, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch g.state {
	case groupStateInit:
		g.state = groupStateRunning
		g.stopReqCh = make(chan shutdownReason, 1)
		g.doneCh = make(chan struct{})
		g.resultErr = nil
		if g.stopPending {
			// Stop raced ahead of this transition; honour it now.
			g.stopPending = false
			g.stopReqCh <- shutdownManualStop
		}
		return g.stopReqCh, g.doneCh, nil
	case groupStateRunning, groupStateStopping:
		return nil, nil, ErrGroupAlreadyStarted
	case groupStateStopped:
		return nil, nil, ErrGroupAlreadyStopped
	default:
		return nil, nil, errors.New("task group in unknown state")
	}
}

func (g *Group) finishLifecycle(doneCh chan struct{}, finalErr error) {
	g.mu.Lock()
	g.resultErr = finalErr
	g.state = groupStateStopped
	g.stopReqCh = nil
	g.stopCleanupCtx = nil
	g.mu.Unlock()
	close(doneCh)
}

// groupRun is the in-flight Start of one Group: run stages, then stop
// whatever was started. Fields that stopTasks writes are only read after
// stopDone is closed.
type groupRun struct {
	g         *Group
	parent    context.Context
	runCtx    context.Context
	runCancel context.CancelFunc
	stopReqCh <-chan shutdownReason
	stages    [][]Task

	startErrs multierr.Error
	stopErrs  multierr.Error
	stopOnce  sync.Once
	stopDone  chan struct{}
	reason    shutdownReason
	stopping  bool
}

type startResult struct {
	idx int
	err error
}

func (r *groupRun) loop() error {
	// A cancelled parent keeps Done readable forever; nil the case after
	// handling it so the select cannot busy-spin while remaining Starts drain.
	ctxDone := r.parent.Done()
	for stageIdx, stage := range r.stages {
		// Honour a stop or parent cancel before launching this stage, including
		// the first: a Stop that arrived before Start must not start members
		// only to tear them down. stopUpTo is stageIdx-1, and -1 means none.
		select {
		case reqReason := <-r.stopReqCh:
			r.beginStop(reqReason, stageIdx-1)
		case <-ctxDone:
			ctxDone = nil
			r.beginStop(shutdownParentCancel, stageIdx-1)
		default:
		}
		if r.stopping {
			break
		}
		r.runStage(stageIdx, stage, &ctxDone)
	}

	if !r.stopping && len(r.stages) > 0 {
		r.beginStop(shutdownNaturalComplete, len(r.stages)-1)
	}
	if r.stopping {
		<-r.stopDone
	}
	return r.finalErr()
}

func (r *groupRun) runStage(stageIdx int, stage []Task, ctxDone *<-chan struct{}) {
	startResults := make(chan startResult, len(stage))
	startedIDs := make([]string, len(stage))
	finished := make([]bool, len(stage))
	for i, t := range stage {
		task := t
		idx := i
		id := task.Identifier()
		startedIDs[idx] = id
		go func() {
			err := execute(r.runCtx, id, task, func(taskCtx context.Context, current Task) error {
				log.Infof("<task> %s starting", current.Identifier())
				return current.Start(taskCtx)
			})
			startResults <- startResult{idx: idx, err: err}
		}()
	}

	var startDeadline <-chan time.Time
	var startTimer *time.Timer
	if stageIdx < len(r.stages)-1 && r.g.opts.startTimeout > 0 {
		startTimer = time.NewTimer(r.g.opts.startTimeout)
		startDeadline = startTimer.C
	}

	remaining := len(stage)
	for remaining > 0 {
		select {
		case reqReason := <-r.stopReqCh:
			r.beginStop(reqReason, stageIdx)
		case <-*ctxDone:
			*ctxDone = nil
			r.beginStop(shutdownParentCancel, stageIdx)
		case <-startDeadline:
			startDeadline = nil
			var stuck []string
			for i, id := range startedIDs {
				if !finished[i] {
					stuck = append(stuck, id)
				}
			}
			sort.Strings(stuck)
			r.startErrs.Add(fmt.Errorf("%w: stage %d still starting: %s", ErrStartTimeout, stageIdx, strings.Join(stuck, ", ")))
			r.beginStop(shutdownTaskFailure, stageIdx)
		case result := <-startResults:
			remaining--
			if result.idx >= 0 && result.idx < len(finished) {
				finished[result.idx] = true
			}
			if result.err == nil {
				continue
			}
			// Teardown-provoked cancellation is not a failure. A Canceled
			// that arrives while runCtx is still live is: the task failed on
			// its own, even if it wraps context.Canceled.
			if errors.Is(result.err, context.Canceled) && r.runCtx.Err() != nil {
				continue
			}
			r.startErrs.Add(result.err)
			r.beginStop(shutdownTaskFailure, stageIdx)
		}
	}
	if startTimer != nil {
		startTimer.Stop()
	}
}

func (r *groupRun) beginStop(stopReason shutdownReason, stopUpTo int) {
	r.stopOnce.Do(func() {
		r.stopping = true
		r.reason = stopReason

		r.g.mu.Lock()
		r.g.state = groupStateStopping
		r.g.mu.Unlock()

		r.runCancel()

		go func() {
			outer := context.Background()
			if stopReason == shutdownManualStop {
				r.g.mu.Lock()
				if r.g.stopCleanupCtx != nil {
					outer = r.g.stopCleanupCtx
				}
				r.g.mu.Unlock()
			}
			stopCtx, stopCancel := r.g.cleanupContext(outer)
			defer stopCancel()
			r.g.stopTasks(stopCtx, &r.stopErrs, stopUpTo)
			close(r.stopDone)
		}()
	})
}

func (r *groupRun) finalErr() error {
	switch r.reason {
	case shutdownTaskFailure, shutdownManualStop, shutdownParentCancel, shutdownNaturalComplete:
		// Include start errors from members that failed on their own while
		// teardown was already in progress. Cancellation-driven exits are
		// filtered before they reach startErrs.
		return errors.Join(r.startErrs.Unwrap(), r.stopErrs.Unwrap())
	default:
		return r.startErrs.Unwrap()
	}
}

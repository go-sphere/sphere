package task

import (
	"context"
	"errors"
	"fmt"
	"reflect"
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
//
// Start returns the member Start and Stop errors joined with errors.Join, or
// nil when there were none. context.Canceled results provoked by the group's
// own teardown (Stop or parent ctx cancellation) are not counted as errors. A
// nil or typed-nil member, or one whose Identifier panics, yields an error
// before anything starts. A Group runs once: a second Start returns
// ErrGroupAlreadyStarted while running and ErrGroupAlreadyStopped afterwards.
func (g *Group) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	stages := g.waves
	if stages == nil {
		stages = [][]Task{g.tasks}
	}
	ids, err := validateTasks(stages)
	if err != nil {
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
		stages:    stages,
		ids:       ids,
		stopDone:  make(chan struct{}),
	}

	finalErr := r.loop()
	g.finishLifecycle(doneCh, finalErr)
	return finalErr
}

// validateTasks rejects nil members and resolves every identifier up front,
// indexing members across stages in order. Group code runs outside a
// recover once the lifecycle has begun, so a member that would panic there
// (a typed nil, or an Identifier that panics) must be refused before then.
func validateTasks(stages [][]Task) ([][]string, error) {
	ids := make([][]string, len(stages))
	idx := 0
	for s, stage := range stages {
		ids[s] = make([]string, len(stage))
		for i, t := range stage {
			if isNilTask(t) {
				return nil, fmt.Errorf("task at index %d is nil", idx)
			}
			id, err := taskIdentifier(t)
			if err != nil {
				return nil, fmt.Errorf("task at index %d: %w", idx, err)
			}
			ids[s][i] = id
			idx++
		}
	}
	return ids, nil
}

func isNilTask(t Task) bool {
	if t == nil {
		return true
	}
	v := reflect.ValueOf(t)
	switch v.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}

func taskIdentifier(t Task) (id string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("identifier panic: %v", r)
		}
	}()
	return t.Identifier(), nil
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
	// ids holds each member's Identifier, resolved once by validateTasks.
	ids [][]string

	startErrs multierr.Error
	stopErrs  multierr.Error
	stopOnce  sync.Once
	stopDone  chan struct{}
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
	startedIDs := r.ids[stageIdx]
	finished := make([]bool, len(stage))
	for i, t := range stage {
		task := t
		idx := i
		id := startedIDs[idx]
		go func() {
			err := execute(r.runCtx, id, task, func(taskCtx context.Context, current Task) error {
				log.Infof("<task> %s starting", id)
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
			if r.stopping {
				// Teardown is already in progress; members are expected to be
				// blocked in Start until their Stop completes, so the timeout
				// would misreport a clean shutdown as ErrStartTimeout.
				continue
			}
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
			finished[result.idx] = true
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
			r.g.stopTasks(stopCtx, r.stages, r.ids, &r.stopErrs, stopUpTo)
			close(r.stopDone)
		}()
	})
}

// finalErr includes start errors from members that failed on their own while
// teardown was already in progress. Cancellation-driven exits are filtered
// before they reach startErrs.
func (r *groupRun) finalErr() error {
	return errors.Join(r.startErrs.Unwrap(), r.stopErrs.Unwrap())
}

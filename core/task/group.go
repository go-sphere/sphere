package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-sphere/sphere/core/task/multierr"
	"github.com/go-sphere/sphere/log"
)

var (
	// ErrGroupAlreadyStarted indicates the group has already entered its run lifecycle.
	ErrGroupAlreadyStarted = errors.New("task group already started")
	// ErrGroupAlreadyStopped indicates the group has already completed its lifecycle.
	ErrGroupAlreadyStopped = errors.New("task group already stopped")
	// ErrGroupNotStarted indicates the group has not been started yet.
	ErrGroupNotStarted = errors.New("task group not started")
)

type groupState uint8

const (
	groupStateInit groupState = iota
	groupStateRunning
	groupStateStopping
	groupStateStopped
)

type shutdownReason uint8

const (
	shutdownNone shutdownReason = iota
	shutdownTaskFailure
	shutdownManualStop
	shutdownParentCancel
)

// GroupOption customizes group runtime behavior.
type GroupOption func(*groupOptions)

type groupOptions struct {
	autoStopTimeout time.Duration
}

// WithAutoStopTimeout configures the timeout used by internal auto-stop cleanup.
// It affects cleanup triggered from Start (task failure, parent cancellation, or manual stop signal).
// A non-positive duration disables timeout and uses context.Background().
func WithAutoStopTimeout(timeout time.Duration) GroupOption {
	return func(o *groupOptions) {
		o.autoStopTimeout = timeout
	}
}

// Group manages the lifecycle of multiple tasks as a coordinated unit.
// It ensures all tasks start together and provides graceful shutdown capabilities.
// The group implements the Task interface, allowing it to be nested within other groups.
type Group struct {
	tasks []Task
	opts  groupOptions

	mu        sync.Mutex
	state     groupState
	stopReqCh chan shutdownReason
	doneCh    chan struct{}
	resultErr error
}

// NewGroup creates a new task group with the provided tasks.
// All tasks will be managed together with coordinated startup and shutdown.
func NewGroup(tasks ...Task) *Group {
	return NewGroupWithOptions(tasks)
}

// NewGroupWithOptions creates a task group with explicit options.
func NewGroupWithOptions(tasks []Task, options ...GroupOption) *Group {
	copied := append([]Task(nil), tasks...)
	opts := groupOptions{}
	for _, option := range options {
		if option == nil {
			continue
		}
		option(&opts)
	}
	return &Group{
		tasks: copied,
		opts:  opts,
		state: groupStateInit,
	}
}

// Identifier returns the group's identifier for logging and debugging purposes.
func (g *Group) Identifier() string {
	return "group"
}

// Start begins all tasks in the group concurrently.
// If any task fails to start, all other tasks will be stopped.
// Returns an error if the group is already started/stopped or if any task fails.
func (g *Group) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	tasks := g.tasks
	for idx, task := range tasks {
		if task == nil {
			return fmt.Errorf("task at index %d is nil", idx)
		}
	}

	g.mu.Lock()
	switch g.state {
	case groupStateInit:
		g.state = groupStateRunning
		g.stopReqCh = make(chan shutdownReason, 1)
		g.doneCh = make(chan struct{})
		g.resultErr = nil
	case groupStateRunning, groupStateStopping:
		g.mu.Unlock()
		return ErrGroupAlreadyStarted
	case groupStateStopped:
		g.mu.Unlock()
		return ErrGroupAlreadyStopped
	default:
		g.mu.Unlock()
		return errors.New("task group in unknown state")
	}
	stopReqCh := g.stopReqCh
	doneCh := g.doneCh
	g.mu.Unlock()

	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	type startResult struct {
		err error
	}
	startResults := make(chan startResult, len(tasks))
	for _, t := range tasks {
		task := t
		go func() {
			err := execute(runCtx, task.Identifier(), task, func(taskCtx context.Context, current Task) error {
				log.Infof("<task> %s starting", task.Identifier())
				return current.Start(taskCtx)
			})
			startResults <- startResult{err: err}
		}()
	}

	var (
		startErrs      multierr.Error
		stopErrs       multierr.Error
		stopOnce       sync.Once
		stopDone       = make(chan struct{})
		reason         shutdownReason
		stopInProgress bool
	)
	beginStop := func(stopReason shutdownReason) {
		stopOnce.Do(func() {
			stopInProgress = true
			reason = stopReason

			g.mu.Lock()
			g.state = groupStateStopping
			g.mu.Unlock()

			runCancel()

			go func() {
				var stopWG sync.WaitGroup
				stopCtx, stopCancel := g.newAutoStopContext()
				defer stopCancel()
				for _, t := range tasks {
					task := t
					stopWG.Go(func() {
						err := execute(stopCtx, task.Identifier(), task, func(taskCtx context.Context, current Task) error {
							log.Infof("<task> %s stopping", task.Identifier())
							return current.Stop(taskCtx)
						})
						if err != nil {
							stopErrs.Add(err)
						}
					})
				}
				stopWG.Wait()
				close(stopDone)
			}()
		})
	}

	remaining := len(tasks)
	for remaining > 0 {
		select {
		case reqReason := <-stopReqCh:
			beginStop(reqReason)
		case <-ctx.Done():
			beginStop(shutdownParentCancel)
		case result := <-startResults:
			remaining--
			if result.err == nil {
				continue
			}
			if errors.Is(result.err, context.Canceled) {
				continue
			}
			startErrs.Add(result.err)
			beginStop(shutdownTaskFailure)
		}
	}

	if stopInProgress {
		<-stopDone
	}

	var finalErr error
	switch reason {
	case shutdownTaskFailure:
		finalErr = errors.Join(startErrs.Unwrap(), stopErrs.Unwrap())
	case shutdownManualStop, shutdownParentCancel:
		finalErr = stopErrs.Unwrap()
	default:
		finalErr = startErrs.Unwrap()
	}

	g.mu.Lock()
	g.resultErr = finalErr
	g.state = groupStateStopped
	g.stopReqCh = nil
	g.mu.Unlock()
	close(doneCh)

	return finalErr
}

// Stop gracefully shuts down all tasks in the group.
// It blocks until shutdown completes or the provided context expires.
// Returns ErrGroupNotStarted when called before Start.
func (g *Group) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	g.mu.Lock()
	state := g.state
	stopReqCh := g.stopReqCh
	doneCh := g.doneCh
	resultErr := g.resultErr
	g.mu.Unlock()

	switch state {
	case groupStateInit:
		return ErrGroupNotStarted
	case groupStateRunning:
		if stopReqCh != nil {
			select {
			case stopReqCh <- shutdownManualStop:
			default:
			}
		}
		return g.waitForDone(ctx, doneCh)
	case groupStateStopping:
		return g.waitForDone(ctx, doneCh)
	case groupStateStopped:
		return resultErr
	default:
		return errors.New("task group in unknown state")
	}
}

// IsStarted reports whether the group has entered its lifecycle.
// It remains true once Start has been invoked successfully.
func (g *Group) IsStarted() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state == groupStateRunning || g.state == groupStateStopping || g.state == groupStateStopped
}

// IsStopped reports whether the group has fully completed shutdown.
func (g *Group) IsStopped() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state == groupStateStopped
}

func (g *Group) waitForDone(ctx context.Context, done <-chan struct{}) error {
	if done == nil {
		return nil
	}
	select {
	case <-done:
		g.mu.Lock()
		defer g.mu.Unlock()
		return g.resultErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *Group) newAutoStopContext() (context.Context, context.CancelFunc) {
	if g.opts.autoStopTimeout <= 0 {
		return context.Background(), func() {}
	}
	return context.WithTimeout(context.Background(), g.opts.autoStopTimeout)
}

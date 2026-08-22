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

var (
	// ErrGroupAlreadyStarted indicates the group has already entered its run lifecycle.
	ErrGroupAlreadyStarted = errors.New("task group already started")
	// ErrGroupAlreadyStopped indicates the group has already completed its lifecycle.
	ErrGroupAlreadyStopped = errors.New("task group already stopped")
	// ErrStartTimeout indicates a non-final stage did not finish Start within
	// the budget set by WithStartTimeout.
	ErrStartTimeout = errors.New("task group start timed out")
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
	// shutdownNaturalComplete is used when every launched Start returned on
	// its own. Stop still runs: it is the cleanup half of the lifecycle, not
	// only an interrupt for tasks that were still blocking in Start.
	shutdownNaturalComplete
)

// GroupOption customizes group runtime behavior.
type GroupOption func(*groupOptions)

type groupOptions struct {
	cleanupTimeout time.Duration
	// cleanupTimeoutSet distinguishes "left at the default" from an explicit
	// non-positive value, which is how a caller asks for an unbounded cleanup.
	cleanupTimeoutSet bool
	startTimeout      time.Duration
}

// defaultCleanupTimeout bounds the ctx handed to each task's Stop when the
// caller configures nothing. Start waits on the cleanup to finish and takes no
// context of its own, so without a bound one task whose Stop honours its context
// but never completes would leave the group stopping — and Start blocked —
// forever. It matches defaultManagerCleanupTimeout, which exists for the same
// reason on the same lifecycle.
const defaultCleanupTimeout = 30 * time.Second

// WithCleanupTimeout configures the timeout applied to the ctx passed to each
// task's Stop() during internal auto-cleanup (task failure / parent cancel /
// manual stop). It does NOT bound Group.Stop(ctx) — callers should pass their
// own context for hard caller-side timeouts. A non-positive duration disables
// the timeout and uses context.Background().
//
// Disabling it is a deliberate choice, not a safe default: Group.Start waits on
// the cleanup to finish and has no context of its own, so an unbounded budget
// lets one task that never returns from Stop keep the group in its stopping
// state permanently. Every later Stop then just times out. That is why the
// default is defaultCleanupTimeout, matching WithManagerCleanupTimeout.
//
// This replaces WithAutoStopTimeout, which was removed without an alias. The
// behaviour is unchanged — the old name never bounded Group.Stop either — so the
// migration is a rename and nothing more.
func WithCleanupTimeout(timeout time.Duration) GroupOption {
	return func(o *groupOptions) {
		o.cleanupTimeout = timeout
		o.cleanupTimeoutSet = true
	}
}

// WithStartTimeout bounds how long a non-final staged group stage may spend
// in Start before the group aborts that stage and tears down everything
// already launched. The last stage is exempt: a long-running server's Start
// is expected not to return until shutdown, so a deadline there would kill
// the process after it was already serving. NewGroup is a single stage, so
// this option has no effect on it.
//
// A non-positive duration disables the bound (the default). The error
// returned on timeout wraps ErrStartTimeout and names the tasks still inside
// Start.
func WithStartTimeout(timeout time.Duration) GroupOption {
	return func(o *groupOptions) {
		o.startTimeout = timeout
	}
}

// Group manages the lifecycle of multiple tasks as a coordinated unit.
// Tasks in one stage start together; staged groups start stages in order and
// stop them in reverse. The group implements the Task interface, allowing it
// to be nested within other groups.
type Group struct {
	tasks []Task
	// waves defines the staged lifecycle order. When nil, all tasks form one
	// stage that starts and stops concurrently. When set, stages start in order
	// (each stage fully started before the next begins) and stop in reverse
	// order (last stage first), each stage draining fully before the previous
	// begins.
	waves [][]Task
	opts  groupOptions

	mu        sync.Mutex
	state     groupState
	stopReqCh chan shutdownReason
	doneCh    chan struct{}
	resultErr error
	// stopPending records a Stop that arrived before Start completed its state
	// transition. Start consumes it so the group tears down immediately instead
	// of running forever: Stop is the only way in, and stopReqCh does not exist
	// until Start creates it, so a dropped early Stop would be unrecoverable.
	stopPending bool
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

// NewStagedGroup creates a task group whose members are organized into ordered
// lifecycle stages. Stages start in order: every task in a stage must finish
// Start successfully before the next stage begins, so a stage can rely on the
// resources its predecessors advertise. Stages stop in reverse order: the last
// stage drains fully before the previous one begins stopping, and tasks within
// a stage stop concurrently. This lets dependents (for example an HTTP server)
// drain before their dependencies (for example a database or cache cleaner) are
// torn down.
//
// Start semantics within a stage match NewGroup: tasks in the same stage start
// concurrently, and a failure in any task tears down every started stage in
// reverse order. Stages whose start never began receive neither Start nor Stop.
// Every task whose Start was invoked is stopped, including one-shot tasks
// whose Start returned on its own. A task that only returns from Start when
// the whole group is stopping (a long-running server, for example) must sit
// in the last stage, or every stage after it is unreachable. Passing a single
// stage is equivalent to NewGroup.
func NewStagedGroup(waves ...[]Task) *Group {
	return NewStagedGroupWithOptions(waves)
}

// NewStagedGroupWithOptions creates a staged task group with explicit options.
// See NewStagedGroup for the staged lifecycle semantics.
func NewStagedGroupWithOptions(waves [][]Task, options ...GroupOption) *Group {
	opts := groupOptions{}
	for _, option := range options {
		if option == nil {
			continue
		}
		option(&opts)
	}

	copiedWaves := make([][]Task, 0, len(waves))
	var flat []Task
	for _, wave := range waves {
		copied := append([]Task(nil), wave...)
		copiedWaves = append(copiedWaves, copied)
		flat = append(flat, copied...)
	}

	return &Group{
		tasks: flat,
		waves: copiedWaves,
		opts:  opts,
		state: groupStateInit,
	}
}

// Identifier returns the group's identifier for logging and debugging purposes.
func (g *Group) Identifier() string {
	return "group"
}

// Start begins the group's tasks in staged order. Tasks within one stage start
// concurrently; the next stage only begins once every task in the current
// stage has finished Start successfully, so a stage can rely on the resources
// its predecessors advertise. If any task fails to start, every stage that
// began is stopped in reverse order. A group without stages (NewGroup) starts
// all tasks concurrently.
//
// Stop is always invoked for every task whose Start was called, including
// when every Start returns on its own (one-shot tasks). Group.Start does not
// return until that cleanup has settled.
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
		if g.stopPending {
			// Stop raced ahead of this transition; honour it now.
			g.stopPending = false
			g.stopReqCh <- shutdownManualStop
		}
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
		idx int
		err error
	}
	stages := g.waves
	if stages == nil {
		stages = [][]Task{tasks}
	}

	var (
		startErrs      multierr.Error
		stopErrs       multierr.Error
		stopOnce       sync.Once
		stopDone       = make(chan struct{})
		reason         shutdownReason
		stopInProgress bool
	)
	beginStop := func(stopReason shutdownReason, stopUpTo int) {
		stopOnce.Do(func() {
			stopInProgress = true
			reason = stopReason

			g.mu.Lock()
			g.state = groupStateStopping
			g.mu.Unlock()

			runCancel()

			go func() {
				stopCtx, stopCancel := g.newCleanupContext()
				defer stopCancel()
				g.stopTasks(stopCtx, &stopErrs, stopUpTo)
				close(stopDone)
			}()
		})
	}

	// Capture ctx.Done() locally so it can be disabled once handled. A cancelled
	// ctx keeps its Done channel readable forever; nil-ing the case prevents the
	// select from busy-spinning while remaining tasks drain.
	ctxDone := ctx.Done()
	for stageIdx, stage := range stages {
		// A stop request or parent cancellation that arrived while the previous
		// stage was starting must be honoured before this stage begins, so its
		// tasks are never launched only to be torn down. The first stage is
		// exempt: a pending request there still observes the historical
		// start-then-stop sequence the stopReqCh race handling relies on.
		if stageIdx > 0 {
			select {
			case reqReason := <-stopReqCh:
				beginStop(reqReason, stageIdx-1)
			case <-ctxDone:
				ctxDone = nil
				beginStop(shutdownParentCancel, stageIdx-1)
			default:
			}
		}
		if stopInProgress {
			break
		}

		startResults := make(chan startResult, len(stage))
		startedIDs := make([]string, len(stage))
		finished := make([]bool, len(stage))
		for i, t := range stage {
			task := t
			idx := i
			id := task.Identifier()
			startedIDs[idx] = id
			go func() {
				err := execute(runCtx, id, task, func(taskCtx context.Context, current Task) error {
					log.Infof("<task> %s starting", current.Identifier())
					return current.Start(taskCtx)
				})
				startResults <- startResult{idx: idx, err: err}
			}()
		}

		var startDeadline <-chan time.Time
		var startTimer *time.Timer
		if stageIdx < len(stages)-1 && g.opts.startTimeout > 0 {
			startTimer = time.NewTimer(g.opts.startTimeout)
			startDeadline = startTimer.C
		}

		remaining := len(stage)
		for remaining > 0 {
			select {
			case reqReason := <-stopReqCh:
				beginStop(reqReason, stageIdx)
			case <-ctxDone:
				ctxDone = nil
				beginStop(shutdownParentCancel, stageIdx)
			case <-startDeadline:
				startDeadline = nil
				var stuck []string
				for i, id := range startedIDs {
					if !finished[i] {
						stuck = append(stuck, id)
					}
				}
				sort.Strings(stuck)
				startErrs.Add(fmt.Errorf("%w: stage %d still starting: %s", ErrStartTimeout, stageIdx, strings.Join(stuck, ", ")))
				beginStop(shutdownTaskFailure, stageIdx)
			case result := <-startResults:
				remaining--
				if result.idx >= 0 && result.idx < len(finished) {
					finished[result.idx] = true
				}
				if result.err == nil {
					continue
				}
				// A context.Canceled result is only expected when the group itself
				// is tearing down: either an internal beginStop cancelled runCtx, or
				// a parent ctx cancellation propagated into runCtx. When runCtx is
				// still live the task failed on its own — even if it wraps
				// context.Canceled — so it must count as a failure and stop the rest
				// rather than being silently swallowed.
				if errors.Is(result.err, context.Canceled) && runCtx.Err() != nil {
					continue
				}
				startErrs.Add(result.err)
				beginStop(shutdownTaskFailure, stageIdx)
			}
		}
		if startTimer != nil {
			startTimer.Stop()
		}
	}

	if !stopInProgress && len(stages) > 0 {
		beginStop(shutdownNaturalComplete, len(stages)-1)
	}
	if stopInProgress {
		<-stopDone
	}

	var finalErr error
	switch reason {
	case shutdownTaskFailure, shutdownManualStop, shutdownParentCancel, shutdownNaturalComplete:
		// Start errors are joined here, not only stop errors. A task that failed
		// for its own reasons while the group was already tearing down used to be
		// discarded on these two paths, so a graceful shutdown reported success
		// even when a task died badly on the way out. Reporting it means an
		// ordinary signal-triggered shutdown can now return non-nil, which callers
		// typically map to a non-zero exit code. This is not an error injected by
		// the teardown itself: startErrs only ever holds results that survived the
		// context.Canceled filter above, so cancellation-driven exits stay nil.
		finalErr = errors.Join(startErrs.Unwrap(), stopErrs.Unwrap())
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
// The provided context only bounds the caller's wait; task Stop calls use the
// cleanup context configured by WithCleanupTimeout.
// Calling Stop before Start records the request and returns nil: a subsequent
// Start tears the group down immediately rather than running unstoppably.
func (g *Group) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	g.mu.Lock()
	if g.state == groupStateInit {
		// Nothing is running yet, so there is nothing to wait for. Record the
		// request under the same lock Start uses for its transition, otherwise a
		// Stop that loses this race would be dropped and the group could never be
		// stopped again.
		g.stopPending = true
		g.mu.Unlock()
		return nil
	}
	state := g.state
	stopReqCh := g.stopReqCh
	doneCh := g.doneCh
	resultErr := g.resultErr
	g.mu.Unlock()

	switch state {
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

// stopTasks stops the group's stages up to and including stage index upToWave,
// honouring staged shutdown order. When waves is nil all tasks form one stage
// that stops concurrently; otherwise stages stop in reverse order (last stage
// first), each stage draining fully before the previous begins, while tasks
// within a stage stop concurrently. Stages beyond upToWave were never started
// and receive no Stop call. The provided ctx (the cleanup context) is the
// shared shutdown budget across every stage.
func (g *Group) stopTasks(ctx context.Context, stopErrs *multierr.Error, upToWave int) {
	waves := g.waves
	if waves == nil {
		waves = [][]Task{g.tasks}
	}

	var inMu sync.Mutex
	inFlight := make(map[string]int)
	mark := func(id string, delta int) {
		inMu.Lock()
		inFlight[id] += delta
		if inFlight[id] <= 0 {
			delete(inFlight, id)
		}
		inMu.Unlock()
	}

	finished := make(chan struct{})
	defer close(finished)
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		go func() {
			select {
			case <-ctx.Done():
				if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
					return
				}
				inMu.Lock()
				var names []string
				for id := range inFlight {
					names = append(names, id)
				}
				inMu.Unlock()
				if len(names) == 0 {
					return
				}
				sort.Strings(names)
				stopErrs.Add(fmt.Errorf("cleanup timed out, still stopping: %s", strings.Join(names, ", ")))
			case <-finished:
			}
		}()
	}

	stopWave := func(wave []Task) {
		var stopWG sync.WaitGroup
		for _, t := range wave {
			task := t
			stopWG.Go(func() {
				id := task.Identifier()
				mark(id, 1)
				defer mark(id, -1)
				err := execute(ctx, id, task, func(taskCtx context.Context, current Task) error {
					log.Infof("<task> %s stopping", current.Identifier())
					return current.Stop(taskCtx)
				})
				if err != nil {
					stopErrs.Add(err)
				}
			})
		}
		stopWG.Wait()
	}
	if upToWave >= len(waves) {
		upToWave = len(waves) - 1
	}
	for i := upToWave; i >= 0; i-- {
		stopWave(waves[i])
	}
}

func (g *Group) newCleanupContext() (context.Context, context.CancelFunc) {
	timeout := g.opts.cleanupTimeout
	if !g.opts.cleanupTimeoutSet {
		timeout = defaultCleanupTimeout
	}
	if timeout <= 0 {
		return context.Background(), func() {}
	}
	return context.WithTimeout(context.Background(), timeout)
}

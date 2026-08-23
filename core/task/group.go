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
//
// An external Stop(ctx) whose ctx has a deadline also bounds member Stop:
// the members see min(ctx deadline, this timeout). This timeout alone applies
// when the group tears itself down (task failure, parent cancel, natural
// complete).
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
// stop them in reverse. Start does not return until every started member has
// been stopped. The group implements Task, so it can be nested or passed to
// boot.NewApplication.
//
// See the package comment for the one-shot and HTTP-process recipes.
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
	// stopCleanupCtx is the ctx from the first Stop that requested shutdown
	// while running. beginStop uses it to bound member Stop; waitForDone still
	// uses each caller's ctx independently.
	stopCleanupCtx context.Context
}

// NewGroup creates a group that starts and stops all tasks concurrently.
// For ordered drain (HTTP before a closer it still uses), use NewStagedGroup.
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

// Stop gracefully shuts down all tasks in the group.
// It blocks until shutdown completes or the provided context expires.
// If ctx has a deadline, member Stop calls are bounded by min(that deadline,
// WithCleanupTimeout). The same ctx still bounds this caller's wait.
// Calling Stop before Start records the request and returns nil: a subsequent
// Start returns without launching members.
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
	if g.state == groupStateRunning && g.stopCleanupCtx == nil {
		g.stopCleanupCtx = ctx
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
		select {
		case <-done:
			g.mu.Lock()
			defer g.mu.Unlock()
			return g.resultErr
		default:
			return ctx.Err()
		}
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
	if upToWave < 0 {
		return
	}
	if upToWave >= len(waves) {
		upToWave = len(waves) - 1
	}
	for i := upToWave; i >= 0; i-- {
		stopWave(waves[i])
	}
}

func (g *Group) cleanupTimeout() time.Duration {
	timeout := g.opts.cleanupTimeout
	if !g.opts.cleanupTimeoutSet {
		timeout = defaultCleanupTimeout
	}
	return timeout
}

func (g *Group) cleanupContext(outer context.Context) (context.Context, context.CancelFunc) {
	if outer == nil {
		outer = context.Background()
	}
	timeout := g.cleanupTimeout()
	if timeout <= 0 {
		return context.WithCancel(outer)
	}
	return context.WithTimeout(outer, timeout)
}

func (g *Group) newCleanupContext() (context.Context, context.CancelFunc) {
	return g.cleanupContext(context.Background())
}

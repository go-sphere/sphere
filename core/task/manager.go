package task

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-sphere/sphere/core/task/multierr"
	"github.com/go-sphere/sphere/log"
)

var (
	// ErrTaskAlreadyExists is returned by Manager.StartTask when name is already running.
	ErrTaskAlreadyExists = errors.New("task already exists")
	// ErrTaskNotFound is returned by lookup helpers when name is not running.
	// StopTask after a successful stop returns nil, not this error.
	ErrTaskNotFound = errors.New("task not found")
)

// ManagerOption customizes manager runtime behavior.
type ManagerOption func(*managerOptions)

type managerOptions struct {
	cleanupTimeout time.Duration
}

// defaultManagerCleanupTimeout bounds the ctx handed to each task's Stop when the
// caller configures nothing. A default matters here because every task is stopped
// before its entry is retired, so Wait blocks on those Stop calls — and Wait takes
// no context of its own. Without a bound, one task whose Stop honours its context
// but never completes on its own would hang Wait for the life of the process.
const defaultManagerCleanupTimeout = 30 * time.Second

// WithManagerCleanupTimeout configures the timeout applied to the ctx passed to
// each task's Stop() inside the manager's internal cleanup goroutine. It is
// independent of the caller's wait context — callers pass their own ctx to
// StopTask/StopAll for hard caller-side timeouts. A non-positive duration disables
// the timeout and uses context.Background(); pass 0 to opt out of the
// defaultManagerCleanupTimeout that NewManager otherwise applies.
//
// The timeout only bounds the context a task's Stop receives. A Stop that ignores
// its context still runs as long as it likes, exactly as in Group.
//
// This replaces WithManagerAutoStopTimeout, which was removed without an alias.
// The knob itself behaves as before; what changed is that NewManager now seeds a
// non-zero default for it.
func WithManagerCleanupTimeout(timeout time.Duration) ManagerOption {
	return func(o *managerOptions) {
		o.cleanupTimeout = timeout
	}
}

type managedTask struct {
	// id uniquely identifies this run of name. Tombstones record it instead of
	// the *managedTask so a finished task's result can be refreshed by a late
	// writer without keeping the user's Task (and whatever it references) alive.
	id     uint64
	name   string
	task   Task
	cancel context.CancelFunc

	doneCh     chan struct{}
	stopDoneCh chan struct{}
	stopOnce   sync.Once

	mu       sync.Mutex
	startErr error
	stopErr  error
}

func newManagedTask(id uint64, name string, task Task, cancel context.CancelFunc) *managedTask {
	return &managedTask{
		id:         id,
		name:       name,
		task:       task,
		cancel:     cancel,
		doneCh:     make(chan struct{}),
		stopDoneCh: make(chan struct{}),
	}
}

func (t *managedTask) setStartErr(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.startErr = err
}

func (t *managedTask) getStartErr() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.startErr
}

func (t *managedTask) setStopErr(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopErr = err
}

func (t *managedTask) getStopErr() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stopErr
}

// Manager is a supervisor for named tasks started later at runtime.
// It is not the process runner: HTTP servers and one-shot jobs belong in a
// Group (typically under boot.Run). Wait holds the registration lock for its
// entire duration, so a long-running task makes Wait a freeze on StartTask.
//
// Stop is treated as the cleanup half of the lifecycle rather than an interrupt
// signal, matching Group: every task the manager starts is stopped exactly once,
// including one that returned from Start on its own without anyone asking it to
// shut down. Earlier releases skipped Stop for those, so a task that allocated in
// Start and then exited early never got the chance to release. Task
// implementations must therefore tolerate Stop after Start has already returned,
// as the Task interface requires.
type Manager struct {
	opts managerOptions

	opsMu  sync.Mutex
	runMu  sync.Mutex
	mu     sync.RWMutex
	nextID uint64
	tasks  map[string]*managedTask
	// tombstones retains the final result of removed tasks so that a StopTask /
	// GetTaskResult call after a task has already exited surfaces its cached
	// result instead of ErrTaskNotFound. Entries are cleared when the same name
	// is re-registered via StartTask.
	//
	// Callers that start tasks under ever-changing names (a per-job UUID, say)
	// would otherwise grow this map without bound, so it is capped at
	// maxTombstones entries with the oldest evicted first; tombstoneOrder tracks
	// that insertion order. An evicted name falls back to ErrTaskNotFound.
	tombstones     map[string]taskResult
	tombstoneOrder []string

	runWG    sync.WaitGroup
	startErr multierr.Error
	stopErr  multierr.Error
}

// maxTombstones bounds how many finished-task results the manager retains. It is
// far above the number of distinct names a typical manager sees, and only matters
// for callers generating a fresh name per task.
const maxTombstones = 1024

// maxRetainedErrors bounds the manager's lifetime start/stop error accumulators.
// They are never reset, so a supervisor restarting a failing task would otherwise
// grow them without limit and make every Wait/StopAll join an ever-longer list.
// The cap mirrors maxTombstones: far above what a healthy manager produces, and
// only reached by callers churning through failures.
const maxRetainedErrors = 1024

// taskResult is a retained result together with the id of the run that produced
// it, so a late writer can refresh its own entry but never overwrite a newer run's.
type taskResult struct {
	owner uint64
	err   error
}

// NewManager creates a new task manager with no initial tasks.
//
// Unless WithManagerCleanupTimeout says otherwise it applies
// defaultManagerCleanupTimeout, so a task whose Stop honours its context is given
// a bounded budget instead of the unbounded one earlier releases used. A Stop
// that legitimately needs longer must raise the bound explicitly, or pass 0 to
// restore an unbounded cleanup context.
func NewManager(options ...ManagerOption) *Manager {
	opts := managerOptions{cleanupTimeout: defaultManagerCleanupTimeout}
	for _, option := range options {
		if option == nil {
			continue
		}
		option(&opts)
	}

	manager := &Manager{
		opts:       opts,
		tasks:      make(map[string]*managedTask),
		tombstones: make(map[string]taskResult),
	}
	manager.startErr.Limit = maxRetainedErrors
	manager.stopErr.Limit = maxRetainedErrors
	return manager
}

// StartTask starts a new task with the given name.
// Returns ErrTaskAlreadyExists if a task with the same name is already running.
// The task runs in its own goroutine and can be stopped individually using StopTask.
// The provided ctx becomes the parent context of this task's run context.
func (m *Manager) StartTask(ctx context.Context, name string, task Task) error {
	if task == nil {
		return errors.New("task is nil")
	}
	if name == "" {
		return errors.New("task name is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	m.runMu.Lock()
	defer m.runMu.Unlock()

	m.opsMu.Lock()
	defer m.opsMu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)

	m.mu.Lock()
	if existing, ok := m.tasks[name]; ok {
		if existing != nil && !isClosed(existing.doneCh) {
			m.mu.Unlock()
			cancel()
			return ErrTaskAlreadyExists
		}
		if existing == nil {
			delete(m.tasks, name)
		}
	}
	m.nextID++
	entry := newManagedTask(m.nextID, name, task, cancel)
	m.dropTombstone(name)
	m.tasks[name] = entry
	m.runWG.Add(1)
	m.mu.Unlock()

	go func() {
		defer m.runWG.Done()
		// runCtx is a child of the caller's ctx, so it stays registered on that
		// parent until cancelled. A task that exits on its own never reaches
		// requestStop, which is the only other place entry.cancel is called, so
		// without this the registration would outlive the task for as long as the
		// parent ctx lives. CancelFunc is idempotent, so the requestStop path is
		// unaffected. Registered after runWG.Done so it runs before it, which lets
		// Wait guarantee every run context has been released.
		defer entry.cancel()
		defer close(entry.doneCh)

		log.Infof("<manager> %s starting", name)
		err := execute(runCtx, name, task, func(startCtx context.Context, current Task) error {
			return current.Start(startCtx)
		})

		// A context.Canceled result is only expected when this task's run context
		// was actually cancelled — by requestStop or by the caller's parent ctx.
		// When runCtx is still live the task failed on its own, even if the error
		// wraps context.Canceled (upstream HTTP/DB clients routinely do), so it
		// must be recorded as a failure rather than silently discarded. This
		// mirrors the same guard in Group.Start.
		if err != nil && !(errors.Is(err, context.Canceled) && runCtx.Err() != nil) {
			entry.setStartErr(err)
			m.startErr.Add(err)
		}

		// Stop is the cleanup half of the lifecycle, not merely an interrupt, so it
		// runs even when the task exited on its own — the same guarantee Group
		// gives, where every member is stopped regardless of whether its Start had
		// already returned. Without this a task that allocated in Start and exited
		// early would never get the chance to release.
		//
		// stopOnce makes this a no-op when StopTask/StopAll already requested the
		// stop; waiting on stopDoneCh then simply joins the in-flight one. Waiting
		// before retiring the entry is what lets Wait promise that every Stop has
		// settled, and it guarantees the tombstone is written with the stop result
		// already in place.
		m.requestStop(entry)
		<-entry.stopDoneCh

		m.removeTaskIfSame(name, entry)
	}()

	return nil
}

// StopTask stops a running task by name.
// If the task has already exited and been removed, its cached result is returned
// from the tombstone instead of ErrTaskNotFound; ErrTaskNotFound is only returned
// for names that were never registered (or were cleared by re-registration).
// Such a task has already had its Stop called by the manager when it exited, so
// the cached result is final rather than a stop that never happened.
// It waits for both Stop and Start goroutines to finish.
//
// Note the consequence for a task that already finished cleanly: its cached
// result is nil, so StopTask returns nil where earlier releases returned
// ErrTaskNotFound. Code using that error to detect "this task is no longer
// running" will now read the nil as success and take the wrong branch. The
// return value cannot distinguish "I stopped it" from "it was already gone";
// use IsRunning before calling, or GetTaskResult, when that difference matters.
// If the caller ctx expires first, StopTask returns ctx.Err(), but internal stopping continues in background.
// The provided context only bounds the caller's wait; the task Stop call uses
// the cleanup context configured by WithManagerCleanupTimeout.
func (m *Manager) StopTask(ctx context.Context, name string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	entry, ok := m.getTask(name)
	if !ok || entry == nil {
		if tomb, result := m.getTombstone(name); tomb {
			return result
		}
		return ErrTaskNotFound
	}

	m.requestStop(entry)

	if err := waitSignalWithContext(ctx, entry.stopDoneCh); err != nil {
		return err
	}
	if err := waitSignalWithContext(ctx, entry.doneCh); err != nil {
		return err
	}

	m.removeTaskIfSame(name, entry)
	return errors.Join(entry.getStartErr(), entry.getStopErr())
}

// StopAll stops all currently running tasks concurrently.
// It waits for all tasks to complete shutdown before returning.
// If the caller ctx expires first, StopAll returns ctx.Err(), but background stops continue.
// The provided context only bounds the caller's wait; task Stop calls use the
// cleanup context configured by WithManagerCleanupTimeout.
// Returns any errors encountered during shutdown and previously collected task run errors.
func (m *Manager) StopAll(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	m.opsMu.Lock()
	defer m.opsMu.Unlock()

	tasks := m.snapshotTasks()

	var stopErrs multierr.Error
	var stopGroup sync.WaitGroup

	for name, entry := range tasks {
		stopGroup.Add(1)
		go func(taskName string, taskEntry *managedTask) {
			defer stopGroup.Done()
			m.requestStop(taskEntry)

			err := waitSignalWithContext(ctx, taskEntry.stopDoneCh)
			if err == nil {
				err = waitSignalWithContext(ctx, taskEntry.doneCh)
			}
			if err != nil {
				stopErrs.Add(err)
				return
			}
			m.removeTaskIfSame(taskName, taskEntry)
		}(name, entry)
	}

	stopGroup.Wait()
	return errors.Join(
		stopErrs.Unwrap(),
		m.resultErr(),
	)
}

// Wait blocks until all started task goroutines have exited. Because every task
// is stopped before its entry is retired, this also waits for each task's Stop to
// settle — a Stop that ignores its context can therefore block Wait, which is why
// NewManager applies defaultManagerCleanupTimeout.
//
// Wait holds the registration lock for its entire duration, so a StartTask
// that races with Wait blocks until Wait returns. A long-running task therefore
// makes Wait a freeze on new registrations; do not Wait from a supervisor that
// still needs to StartTask.
// It returns task run errors and stop errors accumulated over the manager's lifetime.
func (m *Manager) Wait() error {
	m.runMu.Lock()
	defer m.runMu.Unlock()
	m.runWG.Wait()
	return m.resultErr()
}

// IsRunning checks if a task with the given name is currently running.
func (m *Manager) IsRunning(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.tasks[name]
	return ok && entry != nil
}

// GetRunningTasks returns a slice of names of all currently running tasks.
func (m *Manager) GetRunningTasks() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]string, 0, len(m.tasks))
	for name, entry := range m.tasks {
		if entry == nil {
			continue
		}
		list = append(list, name)
	}
	return list
}

// GetTaskResult reports whether the manager knows name, along with the
// accumulated start/stop error recorded for it. It resolves both currently
// registered tasks (returning the error gathered so far, which may be nil while
// still running) and tasks that have already exited but whose result is retained
// in the tombstone. found is false only for names unknown to the manager; a
// known task that has not failed reports found true with a nil err.
func (m *Manager) GetTaskResult(name string) (found bool, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if entry, ok := m.tasks[name]; ok && entry != nil {
		return true, errors.Join(entry.getStartErr(), entry.getStopErr())
	}
	if result, ok := m.tombstones[name]; ok {
		return true, result.err
	}
	return false, nil
}

// GetTaskCount returns the number of currently running tasks.
func (m *Manager) GetTaskCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, entry := range m.tasks {
		if entry != nil {
			count++
		}
	}
	return count
}

func (m *Manager) getTask(name string) (*managedTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.tasks[name]
	if !ok || entry == nil {
		return nil, false
	}
	return entry, true
}

func (m *Manager) snapshotTasks() map[string]*managedTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	copyTasks := make(map[string]*managedTask, len(m.tasks))
	for name, task := range m.tasks {
		if task == nil {
			continue
		}
		copyTasks[name] = task
	}
	return copyTasks
}

// removeTaskIfSame retires expected and records its result. The run goroutine
// calls it once its task's Stop has settled, so the recorded result is already
// complete; StopTask/StopAll may call it again afterwards, which re-records the
// same thing. Either call only ever touches the entry belonging to the same run,
// so a task restarted under the same name is never clobbered by a straggler from
// the previous run.
func (m *Manager) removeTaskIfSame(name string, expected *managedTask) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if current, ok := m.tasks[name]; ok && current == expected {
		delete(m.tasks, name)
	} else if existing, ok := m.tombstones[name]; !ok || existing.owner != expected.id {
		return
	}
	m.putTombstone(name, expected)
}

// putTombstone records expected's result, evicting the oldest entry when the
// retention cap is reached. Callers must hold m.mu.
func (m *Manager) putTombstone(name string, expected *managedTask) {
	result := taskResult{
		owner: expected.id,
		err:   errors.Join(expected.getStartErr(), expected.getStopErr()),
	}
	if _, ok := m.tombstones[name]; !ok {
		for len(m.tombstones) >= maxTombstones && len(m.tombstoneOrder) > 0 {
			oldest := m.tombstoneOrder[0]
			m.tombstoneOrder = m.tombstoneOrder[1:]
			delete(m.tombstones, oldest)
		}
		m.tombstoneOrder = append(m.tombstoneOrder, name)
	}
	m.tombstones[name] = result
}

// dropTombstone forgets a retained result. Callers must hold m.mu.
func (m *Manager) dropTombstone(name string) {
	if _, ok := m.tombstones[name]; !ok {
		return
	}
	delete(m.tombstones, name)
	for i, candidate := range m.tombstoneOrder {
		if candidate == name {
			m.tombstoneOrder = append(m.tombstoneOrder[:i], m.tombstoneOrder[i+1:]...)
			break
		}
	}
}

func (m *Manager) getTombstone(name string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result, ok := m.tombstones[name]
	return ok, result.err
}

func (m *Manager) requestStop(entry *managedTask) {
	if entry == nil {
		return
	}
	entry.stopOnce.Do(func() {
		go func() {
			defer close(entry.stopDoneCh)

			entry.cancel()
			log.Infof("<manager> %s stopping", entry.name)

			stopCtx, stopCancel := m.newCleanupContext()
			defer stopCancel()

			err := execute(stopCtx, entry.name, entry.task, func(currentCtx context.Context, current Task) error {
				return current.Stop(currentCtx)
			})
			if err != nil {
				m.stopErr.Add(err)
			} else {
				log.Infof("<manager> %s stopped", entry.name)
			}
			entry.setStopErr(err)
		}()
	})
}

func (m *Manager) newCleanupContext() (context.Context, context.CancelFunc) {
	if m.opts.cleanupTimeout <= 0 {
		return context.Background(), func() {}
	}
	return context.WithTimeout(context.Background(), m.opts.cleanupTimeout)
}

func (m *Manager) resultErr() error {
	return errors.Join(m.startErr.Unwrap(), m.stopErr.Unwrap())
}

func waitSignalWithContext(ctx context.Context, ch <-chan struct{}) error {
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

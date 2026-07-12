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
	ErrTaskAlreadyExists = errors.New("task already exists")
	ErrTaskNotFound      = errors.New("task not found")
)

// ManagerOption customizes manager runtime behavior.
type ManagerOption func(*managerOptions)

type managerOptions struct {
	cleanupTimeout time.Duration
}

// WithManagerCleanupTimeout configures the timeout applied to the ctx passed to
// each task's Stop() inside the manager's internal cleanup goroutine spawned by
// StopTask / StopAll. It is independent of the caller's wait context — callers
// pass their own ctx to StopTask/StopAll for hard caller-side timeouts. A
// non-positive duration disables the timeout and uses context.Background().
func WithManagerCleanupTimeout(timeout time.Duration) ManagerOption {
	return func(o *managerOptions) {
		o.cleanupTimeout = timeout
	}
}

type managedTask struct {
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

func newManagedTask(name string, task Task, cancel context.CancelFunc) *managedTask {
	return &managedTask{
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

// Manager provides dynamic management of named tasks with concurrent execution.
// It allows starting, stopping, and monitoring individual tasks by name,
// offering more flexibility than the Group type for long-running applications.
type Manager struct {
	opts managerOptions

	opsMu sync.Mutex
	runMu sync.Mutex
	mu    sync.RWMutex
	tasks map[string]*managedTask
	// tombstones retains the final result of removed tasks so that a StopTask /
	// GetTaskResult call after a task has already exited surfaces its cached
	// result instead of ErrTaskNotFound. Entries are cleared when the same name
	// is re-registered via StartTask, bounding growth to the set of distinct
	// task names seen since the last re-registration.
	tombstones map[string]error

	runWG    sync.WaitGroup
	startErr multierr.Error
	stopErr  multierr.Error
}

// NewManager creates a new task manager with no initial tasks.
func NewManager(options ...ManagerOption) *Manager {
	opts := managerOptions{}
	for _, option := range options {
		if option == nil {
			continue
		}
		option(&opts)
	}

	return &Manager{
		opts:       opts,
		tasks:      make(map[string]*managedTask),
		tombstones: make(map[string]error),
	}
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
	entry := newManagedTask(name, task, cancel)

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
	delete(m.tombstones, name)
	m.tasks[name] = entry
	m.runWG.Add(1)
	m.mu.Unlock()

	go func() {
		defer m.runWG.Done()
		defer close(entry.doneCh)

		log.Infof("<manager> %s starting", name)
		err := execute(runCtx, name, task, func(startCtx context.Context, current Task) error {
			return current.Start(startCtx)
		})

		if err != nil && !errors.Is(err, context.Canceled) {
			entry.setStartErr(err)
			m.startErr.Add(err)
		}

		m.removeTaskIfSame(name, entry)
	}()

	return nil
}

// StopTask stops a running task by name.
// If the task has already exited and been removed, its cached result is returned
// from the tombstone instead of ErrTaskNotFound; ErrTaskNotFound is only returned
// for names that were never registered (or were cleared by re-registration).
// It waits for both Stop and Start goroutines to finish.
// If the caller ctx expires first, StopTask returns ctx.Err(), but internal stopping continues in background.
// The provided context only bounds the caller's wait; the task Stop call uses
// the cleanup context configured by WithManagerCleanupTimeout.
func (m *Manager) StopTask(ctx context.Context, name string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	entry, ok := m.getTask(name)
	if !ok || entry == nil {
		if result, tomb := m.getTombstone(name); tomb {
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

// Wait blocks until all started task goroutines have exited.
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

// GetTaskResult returns the accumulated start/stop error for a task by name.
// It resolves both currently registered tasks (returning the error gathered so
// far, which may be nil while still running) and tasks that have already exited
// but whose result is retained in the tombstone. The bool is false only for
// names that are unknown to the manager.
func (m *Manager) GetTaskResult(name string) (error, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if entry, ok := m.tasks[name]; ok && entry != nil {
		return errors.Join(entry.getStartErr(), entry.getStopErr()), true
	}
	if result, ok := m.tombstones[name]; ok {
		return result, true
	}
	return nil, false
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

func (m *Manager) removeTaskIfSame(name string, expected *managedTask) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if current, ok := m.tasks[name]; ok && current == expected {
		m.tombstones[name] = errors.Join(expected.getStartErr(), expected.getStopErr())
		delete(m.tasks, name)
	}
}

func (m *Manager) getTombstone(name string) (error, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result, ok := m.tombstones[name]
	return result, ok
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

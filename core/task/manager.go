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
	autoStopTimeout time.Duration
}

// WithManagerAutoStopTimeout configures timeout for internal task stop operations.
// This timeout is used by manager-triggered stop flows and is independent of caller wait context.
// A non-positive duration disables timeout and uses context.Background().
func WithManagerAutoStopTimeout(timeout time.Duration) ManagerOption {
	return func(o *managerOptions) {
		o.autoStopTimeout = timeout
	}
}

type managedTask struct {
	name   string
	task   Task
	cancel context.CancelFunc

	doneCh     chan struct{}
	stopDoneCh chan struct{}
	stopOnce   sync.Once

	mu      sync.Mutex
	stopErr error
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
	mu    sync.RWMutex
	tasks map[string]*managedTask

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
		opts:  opts,
		tasks: make(map[string]*managedTask),
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
			m.startErr.Add(err)
		}

		m.removeTaskIfSame(name, entry)
	}()

	return nil
}

// StopTask stops a running task by name.
// Returns ErrTaskNotFound if no task with the given name is running.
// It waits for both Stop and Start goroutines to finish.
// If the caller ctx expires first, StopTask returns ctx.Err(), but internal stopping continues in background.
// The task.Stop call itself uses manager-level stop timeout policy (WithManagerAutoStopTimeout).
func (m *Manager) StopTask(ctx context.Context, name string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	entry, ok := m.getTask(name)
	if !ok || entry == nil {
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
	return entry.getStopErr()
}

// StopAll stops all currently running tasks concurrently.
// It waits for all tasks to complete shutdown before returning.
// If the caller ctx expires first, StopAll returns ctx.Err(), but background stops continue.
// The task.Stop calls use manager-level stop timeout policy (WithManagerAutoStopTimeout).
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
// It returns joined task run errors and stop errors collected by the manager.
func (m *Manager) Wait() error {
	m.opsMu.Lock()
	defer m.opsMu.Unlock()
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
		delete(m.tasks, name)
	}
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

			stopCtx, stopCancel := m.newAutoStopContext()
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

func (m *Manager) newAutoStopContext() (context.Context, context.CancelFunc) {
	if m.opts.autoStopTimeout <= 0 {
		return context.Background(), func() {}
	}
	return context.WithTimeout(context.Background(), m.opts.autoStopTimeout)
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

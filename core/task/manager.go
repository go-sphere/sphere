package task

import (
	"context"
	"errors"
	"sync"

	"github.com/go-sphere/sphere/core/task/multierr"
	"github.com/go-sphere/sphere/log"
	"golang.org/x/sync/errgroup"
)

var (
	ErrTaskAlreadyExists = errors.New("task already exists")
	ErrTaskNotFound      = errors.New("task not found")
)

// Manager provides dynamic management of named tasks with concurrent execution.
// It allows starting, stopping, and monitoring individual tasks by name,
// offering more flexibility than the Group type for long-running applications.
type Manager struct {
	tasks sync.Map
	group errgroup.Group
}

// NewManager creates a new task manager with no initial tasks.
func NewManager() *Manager {
	return &Manager{
		tasks: sync.Map{},
		group: errgroup.Group{},
	}
}

// StartTask starts a new task with the given name.
// Returns ErrTaskAlreadyExists if a task with the same name is already running.
// The task runs concurrently and can be stopped individually using StopTask.
func (m *Manager) StartTask(ctx context.Context, name string, task Task) error {
	if _, loaded := m.tasks.LoadOrStore(name, task); loaded {
		return ErrTaskAlreadyExists
	}

	m.group.Go(func() error {
		log.Infof("<manager> %s starting", name)
		defer m.tasks.Delete(name)
		return execute(ctx, name, task, func(ctx context.Context, task Task) error {
			return task.Start(ctx)
		})
	})

	return nil
}

// StopTask stops a running task by name.
// Returns ErrTaskNotFound if no task with the given name is running.
// The task is removed from the manager after successful shutdown.
func (m *Manager) StopTask(ctx context.Context, name string) error {
	value, ok := m.tasks.LoadAndDelete(name)
	if !ok {
		return ErrTaskNotFound
	}
	task := value.(Task)
	log.Infof("<manager> %s stopping", name)
	err := execute(ctx, name, task, func(ctx context.Context, task Task) error {
		return task.Stop(ctx)
	})
	if err != nil {
		return err
	}
	log.Infof("<manager> %s stopped", name)
	return nil
}

// StopAll stops all currently running tasks concurrently.
// It waits for all tasks to complete shutdown before returning.
// Returns any errors encountered during the shutdown process.
func (m *Manager) StopAll(ctx context.Context) error {
	tasks := make(map[string]Task)
	m.tasks.Range(func(key, value interface{}) bool {
		name := key.(string)
		task := value.(Task)
		tasks[name] = task
		m.tasks.Delete(name)
		return true
	})

	var stopErrs multierr.Error
	var stopGroup sync.WaitGroup

	for name, task := range tasks {
		stopGroup.Add(1)
		go func(taskName string, t Task) {
			defer stopGroup.Done()
			log.Infof("<manager> %s stopping", taskName)
			err := execute(ctx, taskName, t, func(ctx context.Context, task Task) error {
				return task.Stop(ctx)
			})
			if err != nil {
				stopErrs.Add(err)
				return
			}
			log.Infof("<manager> %s stopped", taskName)
		}(name, task)
	}

	stopGroup.Wait()
	return errors.Join(
		stopErrs.Unwrap(),
		m.group.Wait(),
	)
}

// Wait blocks until all managed tasks complete execution.
// Returns any error encountered by the running tasks.
func (m *Manager) Wait() error {
	return m.group.Wait()
}

// IsRunning checks if a task with the given name is currently running.
func (m *Manager) IsRunning(name string) bool {
	_, ok := m.tasks.Load(name)
	return ok
}

// GetRunningTasks returns a slice of names of all currently running tasks.
func (m *Manager) GetRunningTasks() []string {
	var tasks []string
	m.tasks.Range(func(key, value interface{}) bool {
		tasks = append(tasks, key.(string))
		return true
	})
	return tasks
}

// GetTaskCount returns the number of currently running tasks.
func (m *Manager) GetTaskCount() int {
	count := 0
	m.tasks.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

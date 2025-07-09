package task

import (
	"context"
	"errors"
	"github.com/TBXark/sphere/core/errors/multierr"
	"github.com/TBXark/sphere/log"
	"golang.org/x/sync/errgroup"
	"sync"
)

var (
	ErrTaskAlreadyExists = errors.New("task already exists")
	ErrTaskNotFound      = errors.New("task not found")
)

type Manager struct {
	tasks sync.Map
	group errgroup.Group
}

func NewManager() *Manager {
	return &Manager{
		tasks: sync.Map{},
		group: errgroup.Group{},
	}
}

func (m *Manager) StartTask(ctx context.Context, name string, task Task) error {
	if _, loaded := m.tasks.LoadOrStore(name, task); loaded {
		return ErrTaskAlreadyExists
	}

	m.group.Go(func() error {
		log.Infof("<Manager> %s starting", name)
		defer m.tasks.Delete(name)
		return execute(ctx, name, task, func(ctx context.Context, task Task) error {
			return task.Start(ctx)
		})
	})

	return nil
}

func (m *Manager) StopTask(ctx context.Context, name string) error {
	value, ok := m.tasks.LoadAndDelete(name)
	if !ok {
		return ErrTaskNotFound
	}
	task := value.(Task)
	log.Infof("<Manager> %s stopping", name)
	err := task.Stop(ctx)
	if err != nil {
		return err
	}
	log.Infof("<Manager> %s stopped", name)
	return nil
}

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
			log.Infof("<Manager> %s stopping", taskName)
			err := execute(ctx, taskName, t, func(ctx context.Context, task Task) error {
				return task.Stop(ctx)
			})
			if err != nil {
				stopErrs.Add(err)
				return
			}
			log.Infof("<Manager> %s stopped", taskName)
		}(name, task)
	}

	stopGroup.Wait()
	return errors.Join(
		stopErrs.Unwrap(),
		m.group.Wait(),
	)
}

func (m *Manager) Wait() error {
	return m.group.Wait()
}

func (m *Manager) IsRunning(name string) bool {
	_, ok := m.tasks.Load(name)
	return ok
}

func (m *Manager) GetRunningTasks() []string {
	var tasks []string
	m.tasks.Range(func(key, value interface{}) bool {
		tasks = append(tasks, key.(string))
		return true
	})
	return tasks
}

func (m *Manager) GetTaskCount() int {
	count := 0
	m.tasks.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

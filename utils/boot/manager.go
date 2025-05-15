package boot

import (
	"context"
	"errors"
	"fmt"
	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
	"golang.org/x/sync/errgroup"
	"sync"
)

var (
	ErrTaskAlreadyExists = errors.New("task already exists")
	ErrTaskNotFound      = errors.New("task not found")
)

type Manager struct {
	mu           sync.RWMutex
	tasks        map[string]Task
	runningGroup errgroup.Group
}

func NewManager() *Manager {
	return &Manager{
		tasks: make(map[string]Task),
	}
}

func (m *Manager) RunTask(ctx context.Context, name string, task Task, ignoreStartError bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tasks[name]; ok {
		return ErrTaskAlreadyExists
	}
	m.tasks[name] = task
	m.runningGroup.Go(func() error {
		log.Infof("<Manager> %s starting", name)
		err := execute(ctx, name, task, func(ctx context.Context, task Task) error {
			return task.Start(ctx)
		})
		if err != nil {
			logTaskError(task, name, err)
			if ignoreStartError {
				return nil
			}
			return err
		}
		log.Infof("<Manager> %s started", name)
		return nil
	})
	return nil
}

func (m *Manager) StopTask(ctx context.Context, name string) error {
	m.mu.Lock()
	task, ok := m.tasks[name]
	if !ok {
		m.mu.Unlock()
		return ErrTaskNotFound
	}
	delete(m.tasks, name)
	m.mu.Unlock()
	log.Infof("<Manager> %s stopping", name)
	err := task.Stop(ctx)
	if err != nil {
		return err
	}
	log.Infof("<Manager> %s stopped", name)
	return nil
}

func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.Lock()
	tasks := make(map[string]Task, len(m.tasks))
	for name, task := range m.tasks {
		tasks[name] = task
		delete(m.tasks, name)
	}
	m.mu.Unlock()

	var stopGroup sync.WaitGroup
	var stopErrs ErrCollection
	for name, task := range tasks {
		stopGroup.Add(1)
		go func() {
			err := execute(ctx, name, task, func(ctx context.Context, task Task) error {
				return task.Stop(ctx)
			})
			stopErrs.Add(err)
		}()
	}
	return errors.Join(
		stopErrs.Err(),
		m.runningGroup.Wait(),
	)
}

func (m *Manager) Identifier() string {
	return "task_manager"
}

func (m *Manager) Start(ctx context.Context) error {
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	return m.StopAll(ctx)
}

func (m *Manager) Wait() error {
	return m.runningGroup.Wait()
}

func (m *Manager) IsRunning(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.tasks[name]
	return ok
}

type ErrCollection struct {
	mu   sync.Mutex
	errs []error
}

func (ec *ErrCollection) Add(err error) {
	if err == nil {
		return
	}
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.errs = append(ec.errs, err)
}

func (ec *ErrCollection) Err() error {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	if len(ec.errs) == 0 {
		return nil
	}
	return errors.Join(ec.errs...)
}

func execute(ctx context.Context, name string, task Task, run func(ctx context.Context, task Task) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logTaskPanic(task, name, r)
			err = fmt.Errorf("%s panic: %v", name, r)
		}
	}()
	err = run(ctx, task)
	if err != nil {
		logTaskError(task, name, err)
		return
	}
	return
}

func logTaskPanic(task Task, name string, reason any) {
	log.Errorw(
		fmt.Sprintf("%s panic", name),
		logfields.String("task", task.Identifier()),
		logfields.Any("recover", reason),
	)
}

func logTaskError(task Task, name string, err error) {
	log.Errorw(
		fmt.Sprintf("%s error", name),
		logfields.String("task", task.Identifier()),
		logfields.Error(err),
	)
}

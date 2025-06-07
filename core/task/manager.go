package task

import (
	"context"
	"errors"
	"sync"

	"github.com/TBXark/sphere/log"
	"golang.org/x/sync/errgroup"
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

type Options struct {
	stopGroupOnError bool
}

type Option = func(*Options)

func WithStopGroupOnError() Option {
	return func(opts *Options) {
		opts.stopGroupOnError = true
	}
}

func (m *Manager) StartTask(ctx context.Context, name string, task Task, options ...Option) error {
	m.mu.Lock()
	if _, ok := m.tasks[name]; ok {
		m.mu.Unlock()
		return ErrTaskAlreadyExists
	}
	m.tasks[name] = task
	m.mu.Unlock()

	opts := &Options{}
	for _, opt := range options {
		opt(opts)
	}

	m.runningGroup.Go(func() error {
		log.Infof("<Manager> %s starting", name)
		err := execute(ctx, name, task, func(ctx context.Context, task Task) error {
			return task.Start(ctx)
		})
		if err != nil {
			logTaskError(task, name, err)
			if opts.stopGroupOnError {
				return err
			}
			return nil
		}
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

	var stopErrs ErrCollection
	var stopGroup sync.WaitGroup
	for name, task := range tasks {
		stopGroup.Add(1)
		go func() {
			defer stopGroup.Done()
			log.Infof("<Manager> %s stopping", name)
			err := execute(ctx, name, task, func(ctx context.Context, task Task) error {
				return task.Stop(ctx)
			})
			if err != nil {
				stopErrs.Add(err)
				return
			}
			log.Infof("<Manager> %s stopped", name)
		}()
	}
	stopGroup.Wait()

	return errors.Join(
		stopErrs.Err(),
		m.runningGroup.Wait(),
	)
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

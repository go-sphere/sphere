package boot

import (
	"context"
	"errors"
	"fmt"
	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
	"sync"
)

var (
	ErrTaskAlreadyExists = errors.New("task already exists")
	ErrTaskNotFound      = errors.New("task not found")
)

type Manager struct {
	mu           sync.RWMutex
	tasks        map[string]Task
	runningErrs  ErrCollection
	runningGroup sync.WaitGroup
}

func NewManager() *Manager {
	return &Manager{
		tasks: make(map[string]Task),
	}
}

func (m *Manager) RunTask(ctx context.Context, name string, task Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tasks[name]; ok {
		return ErrTaskAlreadyExists
	}
	m.tasks[name] = task
	m.runningGroup.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logTaskPanic(task, name, r)
				err := fmt.Errorf("%s panic: %v", name, r)
				m.runningErrs.add(err)
			}
		}()
		defer func() {
			m.mu.Lock()
			delete(m.tasks, name)
			m.mu.Unlock()
		}()
		defer m.runningGroup.Done()
		log.Infof("<Manager> %s starting", name)
		if err := task.Start(ctx); err != nil {
			logTaskError(task, name, err)
			m.runningErrs.add(err)
			return
		}
		log.Infof("<Manager> %s finished", name)
	}()
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
		go func(name string, task Task) {
			defer func() {
				if r := recover(); r != nil {
					logTaskPanic(task, name, r)
					err := fmt.Errorf("%s panic: %v", name, r)
					stopErrs.add(err)
				}
			}()
			defer stopGroup.Done()
			log.Infof("<Manager> %s stopping", name)
			if err := task.Stop(ctx); err != nil {
				logTaskError(task, name, err)
				stopErrs.add(err)
				return
			}
			log.Infof("<Manager> %s stopped", name)
		}(name, task)
	}
	m.runningGroup.Wait()
	return stopErrs.err()
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
	m.runningGroup.Wait()
	return m.runningErrs.err()
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

func (ec *ErrCollection) add(err error) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.errs = append(ec.errs, err)
}

func (ec *ErrCollection) err() error {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	if len(ec.errs) == 0 {
		return nil
	}
	return errors.Join(ec.errs...)
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

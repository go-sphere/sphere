package testtask

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-sphere/sphere/core/task"
)

var _ task.Task = (*AutoClose)(nil)

type AutoClose struct{}

func NewAutoClose() *AutoClose {
	return &AutoClose{}
}

func (a AutoClose) Identifier() string {
	return "autoclose"
}

func (a AutoClose) Start(ctx context.Context) error {
	time.Sleep(3 * time.Second)
	return errors.New("simulated error for autoclose task")
}

func (a AutoClose) Stop(ctx context.Context) error {
	return nil
}

type AutoPanic struct{}

func NewAutoPanic() *AutoPanic {
	return &AutoPanic{}
}

func (a AutoPanic) Identifier() string {
	return "autopanic"
}

func (a AutoPanic) Start(ctx context.Context) error {
	time.Sleep(3 * time.Second)
	panic("simulated panic for autopanic task")
}

func (a AutoPanic) Stop(ctx context.Context) error {
	return nil
}

type ServerExample struct {
	server *http.Server
}

func NewServerExample() *ServerExample {
	return &ServerExample{
		server: &http.Server{
			Addr: ":0",
		},
	}
}

func (s *ServerExample) Identifier() string {
	return "serverexample"
}

func (s *ServerExample) Start(ctx context.Context) error {
	return s.server.ListenAndServe()
}

func (s *ServerExample) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

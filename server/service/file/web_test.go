package file

import (
	"context"
	"errors"
	"io/fs"
	"sync"
	"testing"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/cache"
	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/core/task/tasktest"
	"github.com/go-sphere/sphere/storage"
)

var _ task.Task = (*Web)(nil)
var _ httpx.Engine = (*stubEngine)(nil)

func TestWebLifecycleContract(t *testing.T) {
	tasktest.AssertLifecycleContract(t, func() task.Task {
		storage, err := NewLocalFileService(LocalFileServiceConfig{
			RootDir:    t.TempDir(),
			PublicBase: "http://127.0.0.1/",
		})
		if err != nil {
			t.Fatalf("NewLocalFileService: %v", err)
		}
		return NewWebServer(newStubEngine(), storage)
	})
}

func TestWebStopReleasesOwnedUploadTokenCache(t *testing.T) {
	t.Parallel()

	adapter, err := NewLocalFileService(LocalFileServiceConfig{
		RootDir:    t.TempDir(),
		PublicBase: "http://127.0.0.1/",
	})
	if err != nil {
		t.Fatalf("NewLocalFileService: %v", err)
	}
	web := NewWebServer(newStubEngine(), adapter)
	ctx := context.Background()

	if _, err := adapter.GenerateUploadAuth(ctx, storage.UploadAuthRequest{FileName: "a.txt"}); err != nil {
		t.Fatalf("GenerateUploadAuth before Stop: %v", err)
	}

	if err := web.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if _, err := adapter.GenerateUploadAuth(ctx, storage.UploadAuthRequest{FileName: "b.txt"}); !errors.Is(err, cache.ErrClosed) {
		t.Fatalf("GenerateUploadAuth after Stop = %v, want cache.ErrClosed", err)
	}

	// A task can be stopped repeatedly; the owned cache must not be closed
	// twice or turn the second Stop into an error.
	if err := web.Stop(ctx); err != nil {
		t.Fatalf("second Stop = %v, want nil", err)
	}
}

func TestWeb_Identifier(t *testing.T) {
	t.Parallel()
	web := NewWebServer(newStubEngine(), nil)
	if got := web.Identifier(); got != "file" {
		t.Fatalf("web.Identifier() = %q, want file", got)
	}
}

// stubEngine is an HTTP-style Engine: Start ignores context and only returns
// after Stop, matching ListenAndServe. Stop is idempotent and safe before Start.
type stubEngine struct {
	stopOnce sync.Once
	stopped  chan struct{}
}

func newStubEngine() *stubEngine {
	return &stubEngine{stopped: make(chan struct{})}
}

func (e *stubEngine) Use(...httpx.Middleware) {}

func (e *stubEngine) Group(string, ...httpx.Middleware) httpx.Router {
	return stubRouter{}
}

func (e *stubEngine) Start() error {
	<-e.stopped
	return nil
}

func (e *stubEngine) Stop(context.Context) error {
	e.stopOnce.Do(func() { close(e.stopped) })
	return nil
}

func (e *stubEngine) IsRunning() bool { return false }

type stubRouter struct{}

func (stubRouter) Use(...httpx.Middleware)                        {}
func (stubRouter) Handle(string, string, httpx.Handler)           {}
func (stubRouter) Any(string, httpx.Handler)                      {}
func (stubRouter) Static(string, string)                          {}
func (stubRouter) StaticFS(string, fs.FS)                         {}
func (stubRouter) BasePath() string                               { return "/" }
func (stubRouter) Group(string, ...httpx.Middleware) httpx.Router { return stubRouter{} }
func (stubRouter) SupportsRouterFeature(httpx.RouterFeature) bool { return true }
func (stubRouter) GET(string, httpx.Handler)                      {}
func (stubRouter) POST(string, httpx.Handler)                     {}
func (stubRouter) PUT(string, httpx.Handler)                      {}
func (stubRouter) DELETE(string, httpx.Handler)                   {}
func (stubRouter) PATCH(string, httpx.Handler)                    {}
func (stubRouter) HEAD(string, httpx.Handler)                     {}
func (stubRouter) OPTIONS(string, httpx.Handler)                  {}

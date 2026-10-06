package file

import (
	"context"
	"errors"
	"io/fs"
	"sync"
	"sync/atomic"
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

// TestWebStartNilFileServer pins that Start reports a nil FileServer as an
// error instead of panicking on the nil receiver.
func TestWebStartNilFileServer(t *testing.T) {
	t.Parallel()
	engine := newStubEngine()
	web := NewWebServer(engine, nil)
	if err := web.Start(context.Background()); err == nil {
		t.Fatal("Start with a nil FileServer = nil, want an error")
	}
	if n := engine.groups.Load(); n != 0 {
		t.Fatalf("Start with a nil FileServer registered %d route groups, want 0", n)
	}
	if err := web.Stop(context.Background()); err != nil {
		t.Fatalf("Stop after failed Start: %v", err)
	}
}

// TestWebStartRegistersRoutesOnce pins that a repeated Start does not
// register the upload and download routes a second time (most routers panic
// on a duplicate route).
func TestWebStartRegistersRoutesOnce(t *testing.T) {
	t.Parallel()
	adapter, err := NewLocalFileService(LocalFileServiceConfig{
		RootDir:    t.TempDir(),
		PublicBase: "http://127.0.0.1/",
	})
	if err != nil {
		t.Fatalf("NewLocalFileService: %v", err)
	}
	engine := newStubEngine()
	web := NewWebServer(engine, adapter)
	// Stop first so each Start returns immediately from the stub engine.
	if err := web.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	for range 2 {
		if err := web.Start(context.Background()); err != nil {
			t.Fatalf("Start: %v", err)
		}
	}
	if n := engine.groups.Load(); n != 2 {
		t.Fatalf("route groups after two Starts = %d, want 2 (uploader + downloader once)", n)
	}
}

// stubEngine is an HTTP-style Engine: Start ignores context and only returns
// after Stop, matching ListenAndServe. Stop is idempotent and safe before Start.
type stubEngine struct {
	stopOnce sync.Once
	stopped  chan struct{}
	groups   atomic.Int32 // Group calls, i.e. route registrations by Start
}

func newStubEngine() *stubEngine {
	return &stubEngine{stopped: make(chan struct{})}
}

func (e *stubEngine) Use(...httpx.Middleware) {}

func (e *stubEngine) Group(string, ...httpx.Middleware) httpx.Router {
	e.groups.Add(1)
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

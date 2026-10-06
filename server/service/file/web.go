package file

import (
	"context"
	"errors"
	"sync"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/storage/fileserver"
	"github.com/go-sphere/sphere/storage/local"
)

var errNilFileServer = errors.New("file: Web has a nil FileServer: pass one to NewWebServer")

// Web is a task.Task wrapping an httpx.Engine and a fileserver.FileServer.
// Construct it with NewWebServer. It takes ownership of both: Stop stops the
// engine and closes the FileServer.
type Web struct {
	engine  httpx.Engine
	storage *fileserver.FileServer

	registerOnce sync.Once
}

// NewWebServer wraps engine and storage as a task.Task. Routes are registered
// on engine when Start runs, not here. Start fails if storage is nil; the
// engine's address and options decide where the service listens. The URLs the
// FileServer issues (its PutBase and GetBase) must point at this engine's root.
func NewWebServer(engine httpx.Engine, storage *fileserver.FileServer) *Web {
	return &Web{
		engine:  engine,
		storage: storage,
	}
}

// LocalFileServiceConfig configures a local-disk fileserver adapter.
// RootDir is the filesystem root; PublicBase is the public URL prefix for both upload and download.
// RootDir is required and is created if missing. PublicBase must be an absolute
// URL of the Web engine's root, such as "https://files.example.com/".
type LocalFileServiceConfig struct {
	RootDir    string `json:"root_dir" yaml:"root_dir"`
	PublicBase string `json:"public_base" yaml:"public_base"`
}

// NewLocalFileService builds a local-disk CDN adapter with an in-memory byte cache and 3600s Cache-Control.
//
// The in-memory token cache has no other owner, so it is marked as owned: the
// adapter's Close releases it, and Web.Stop calls that when the service stops.
// A caller that builds the adapter but never wraps it in a Web is responsible
// for calling Close itself.
func NewLocalFileService(conf LocalFileServiceConfig) (*fileserver.FileServer, error) {
	client, err := local.NewClient(local.Config{
		RootDir: conf.RootDir,
	})
	if err != nil {
		return nil, err
	}
	tokens := memory.NewByteCache()
	adapter, err := fileserver.NewCDNAdapter(
		fileserver.Config{
			PutBase: conf.PublicBase,
			GetBase: conf.PublicBase,
		},
		tokens,
		client,
		fileserver.WithCacheControl(3600),
		fileserver.WithOwnedCache(),
	)
	if err != nil {
		_ = tokens.Close()
		return nil, err
	}
	return adapter, nil
}

// Identifier returns the service identifier for the file web server.
func (w *Web) Identifier() string {
	return "file"
}

// Start registers upload and download handlers and starts the engine. It does not configure CORS.
// It blocks for as long as engine.Start does and returns its result; ctx is not
// used, so stop the service with Stop. A nil FileServer fails with an error.
// Routes are registered on the first Start only; a later Start just calls
// engine.Start again, whose result decides whether that is a restart (for
// example httpx.ErrEngineClosed after Stop).
func (w *Web) Start(ctx context.Context) error {
	if w.storage == nil {
		return errNilFileServer
	}
	w.registerOnce.Do(func() {
		w.storage.RegisterFileUploader(w.engine.Group("/"))
		w.storage.RegisterFileDownloader(w.engine.Group("/"))
	})
	return w.engine.Start()
}

// Stop shuts down the engine before closing resources owned by the adapter.
// A caller-supplied cache remains open unless WithOwnedCache was set.
func (w *Web) Stop(ctx context.Context) error {
	stopErr := w.engine.Stop(ctx)
	if w.storage == nil {
		return stopErr
	}
	return errors.Join(stopErr, w.storage.Close())
}

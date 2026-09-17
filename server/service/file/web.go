// Package file is a task.Task wrapping an httpx.Engine and
// fileserver.FileServer. It is not an S3 API.
//
// PUT /:key uploads (one-time cache token). GET /*filename downloads.
// NewLocalFileService builds a local-disk CDN adapter with an in-memory byte
// cache and 3600s Cache-Control. Identifier is "file". Start does not
// configure CORS — it only registers upload/download and engine.Start.
package file

import (
	"context"
	"errors"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/storage/fileserver"
	"github.com/go-sphere/sphere/storage/local"
)

// Web is a task.Task wrapping an httpx.Engine and a fileserver.FileServer.
type Web struct {
	engine  httpx.Engine
	storage *fileserver.FileServer
}

// NewWebServer wraps engine and storage as a task.Task.
func NewWebServer(engine httpx.Engine, storage *fileserver.FileServer) *Web {
	return &Web{
		engine:  engine,
		storage: storage,
	}
}

// LocalFileServiceConfig configures a local-disk fileserver adapter.
// RootDir is the filesystem root; PublicBase is the public URL prefix for both upload and download.
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
func (w *Web) Start(ctx context.Context) error {
	w.storage.RegisterFileUploader(w.engine.Group("/"))
	w.storage.RegisterFileDownloader(w.engine.Group("/"))
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

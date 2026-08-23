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
func NewLocalFileService(conf LocalFileServiceConfig) (*fileserver.FileServer, error) {
	client, err := local.NewClient(local.Config{
		RootDir: conf.RootDir,
	})
	if err != nil {
		return nil, err
	}
	adapter, err := fileserver.NewCDNAdapter(
		fileserver.Config{
			PutBase: conf.PublicBase,
			GetBase: conf.PublicBase,
		},
		memory.NewByteCache(),
		client,
		fileserver.WithCacheControl(3600),
	)
	if err != nil {
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

// Stop gracefully shuts down the file web server.
func (w *Web) Stop(ctx context.Context) error {
	return w.engine.Stop(ctx)
}

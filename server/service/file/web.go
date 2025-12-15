package file

import (
	"context"
	"net/http"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/cache/memory"
	"github.com/go-sphere/sphere/server/middleware/cors"
	"github.com/go-sphere/sphere/storage/fileserver"
	"github.com/go-sphere/sphere/storage/local"
)

// HTTPConfig contains HTTP server configuration for the file service.
type HTTPConfig struct {
	Cors []string `json:"cors" yaml:"cors"`
}

// Config contains the complete configuration for the file web service.
type Config struct {
	HTTP HTTPConfig `json:"http" yaml:"http"`
}

// Web provides a file upload and download web service with S3-compatible storage.
type Web struct {
	config  *Config
	engine  httpx.Engine
	server  *http.Server
	storage *fileserver.S3Adapter
}

// NewWebServer creates a new file web server with the given configuration and storage adapter.
func NewWebServer(config *Config, engine httpx.Engine, storage *fileserver.S3Adapter) *Web {
	return &Web{
		config:  config,
		engine:  engine,
		storage: storage,
	}
}

// NewLocalFileService creates a new S3Adapter configured for local file storage.
// It sets up the local storage client and wraps it with caching and S3-compatible interface.
func NewLocalFileService(config *local.Config) (*fileserver.S3Adapter, error) {
	client, err := local.NewClient(config)
	if err != nil {
		return nil, err
	}
	adapter := fileserver.NewS3Adapter(
		&fileserver.Config{PublicBase: config.PublicBase},
		memory.NewByteCache(),
		client,
	)
	return adapter, nil
}

// Identifier returns the service identifier for the file web server.
func (w *Web) Identifier() string {
	return "file"
}

// Start begins serving the file web server with upload and download endpoints.
// It configures CORS, registers file upload/download handlers, and starts the HTTP server.
func (w *Web) Start(ctx context.Context) error {
	w.engine.Use(cors.NewCORS())
	w.storage.RegisterPutFileUploader(w.engine.Group("/"))
	w.storage.RegisterFileDownloader(w.engine.Group("/"), fileserver.WithCacheControl(3600))
	return w.engine.Start()
}

// Stop gracefully shuts down the file web server.
func (w *Web) Stop(ctx context.Context) error {
	return httpx.Close(ctx, w.server)
}

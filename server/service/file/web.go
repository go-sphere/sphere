package file

import (
	"context"
	"github.com/TBXark/sphere/cache/memory"
	"github.com/TBXark/sphere/storage/local"
	"net/http"

	"github.com/TBXark/sphere/server/ginx"
	"github.com/TBXark/sphere/server/route/cors"
	"github.com/TBXark/sphere/storage/fileserver"
	"github.com/gin-gonic/gin"
)

type HTTPConfig struct {
	Address string   `json:"address" yaml:"address"`
	Cors    []string `json:"cors" yaml:"cors"`
}

type Config struct {
	HTTP HTTPConfig `json:"http" yaml:"http"`
}

type Web struct {
	config  *Config
	server  *http.Server
	storage *fileserver.S3Adapter
}

func NewWebServer(config *Config, storage *fileserver.S3Adapter) *Web {
	return &Web{
		config:  config,
		storage: storage,
	}
}

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

func (w *Web) Identifier() string {
	return "file"
}

func (w *Web) Start(ctx context.Context) error {
	engine := gin.Default()
	cors.Setup(engine, w.config.HTTP.Cors)
	w.storage.RegisterPutFileUploader(engine)
	w.storage.RegisterFileDownloader(engine, fileserver.WithCacheControl(3600))
	w.server = &http.Server{
		Addr:    w.config.HTTP.Address,
		Handler: engine.Handler(),
	}
	return ginx.Start(w.server)
}

func (w *Web) Stop(ctx context.Context) error {
	return ginx.Close(ctx, w.server)
}

package file

import (
	"context"
	"net/http"

	"github.com/TBXark/sphere/layout/internal/pkg/file"
	"github.com/TBXark/sphere/server/ginx"
	"github.com/TBXark/sphere/server/route/cors"
	"github.com/TBXark/sphere/storage/fileserver"
	"github.com/gin-gonic/gin"
)

type Web struct {
	config  *Config
	server  *http.Server
	storage *file.Service
}

func NewWebServer(config *Config, storage *file.Service) *Web {
	return &Web{
		config:  config,
		storage: storage,
	}
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

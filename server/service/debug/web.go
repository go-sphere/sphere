package debug

import (
	"context"
	"net/http"

	"github.com/TBXark/sphere/server/ginx"
	"github.com/gin-gonic/gin"
)

type HTTPConfig struct {
	Address string `json:"address" yaml:"address"`
}

type Config struct {
	HTTP HTTPConfig `json:"http" yaml:"http"`
}

type Web struct {
	config *Config
	server *http.Server
}

func NewWebServer(config *Config) *Web {
	return &Web{
		config: config,
	}
}

func (w *Web) Identifier() string {
	return "pprof"
}

func (w *Web) Start(ctx context.Context) error {
	if w.config == nil {
		return nil
	}

	engine := gin.Default()
	SetupPProf(engine)

	w.server = &http.Server{
		Addr:    w.config.HTTP.Address,
		Handler: engine,
	}
	return ginx.Start(w.server)
}

func (w *Web) Stop(ctx context.Context) error {
	return ginx.Close(ctx, w.server)
}

package debug

import (
	"context"
	"net/http"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/go-sphere/sphere/server/ginx"
)

// HTTPConfig contains HTTP server configuration for the debug service.
type HTTPConfig struct {
	Address string `json:"address" yaml:"address"`
}

// Config contains the complete configuration for the debug web service.
type Config struct {
	HTTP HTTPConfig `json:"http" yaml:"http"`
}

// Web provides a debug web server with pprof endpoints for performance profiling.
type Web struct {
	config *Config
	server *http.Server
}

// NewWebServer creates a new debug web server instance with the given configuration.
func NewWebServer(config *Config) *Web {
	return &Web{
		config: config,
	}
}

// Identifier returns the service identifier for the debug web server.
func (w *Web) Identifier() string {
	return "pprof"
}

// Start begins serving the debug web server with pprof endpoints.
// It returns nil immediately if no configuration is provided.
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

// Stop gracefully shuts down the debug web server.
func (w *Web) Stop(ctx context.Context) error {
	return ginx.Close(ctx, w.server)
}

// SetupPProf registers pprof debugging endpoints on the given router.
// It provides CPU, memory, and other performance profiling endpoints.
func SetupPProf(route gin.IRouter, prefixOptions ...string) {
	pprof.Register(route, prefixOptions...)
}

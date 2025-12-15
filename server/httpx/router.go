package httpx

import (
	"context"
	"io/fs"
	"net/http"
)

// Handler is the canonical function signature for framework adapters.
type Handler func(Context) error

// ErrorHandler receives the terminal error from a Handler.
type ErrorHandler func(Context, error)

// MiddlewareScope attaches middleware to the current scope.
type MiddlewareScope interface {
	Use(...Middleware)
}

// Registrar registers handlers on a router scope.
type Registrar interface {
	Handle(method, path string, h Handler)
	Any(path string, h Handler)
	Static(prefix, root string)
	StaticFS(prefix string, fs fs.FS)
}

// Router is a full-featured route scope.
type Router interface {
	MiddlewareScope
	Registrar
	Group(prefix string, m ...Middleware) Router
}

// Engine is the entrypoint: it can serve HTTP, apply global middleware,
// and create groups, but cannot register routes directly.
type Engine interface {
	MiddlewareScope
	Start() error
	Stop(ctx context.Context) error
	Group(prefix string, m ...Middleware) Router
}

// Config controls router adapter creation.
type Config[E any] struct {
	ErrorHandler ErrorHandler
	Middleware   MiddlewareChain
	Engine       E // framework-specific passthrough (e.g., *gin.Engine, *fiber.App, *echo.Echo, *chi.Mux)
}

// Option defines a functional option for configuring a Config instance.
type Option[E any] func(*Config[E])

// NewConfig builds a Config with the given options.
func NewConfig[E any](opts ...Option[E]) *Config[E] {
	conf := &Config[E]{}
	for _, opt := range opts {
		if opt != nil {
			opt(conf)
		}
	}
	if conf.ErrorHandler == nil {
		conf.ErrorHandler = func(ctx Context, err error) {
			if !ctx.IsAborted() {
				ctx.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
		}
	}
	return conf
}

// Apply iterates through the provided options and applies each non-nil option to the Config instance.
func (c *Config[E]) Apply(opts ...Option[E]) {
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
}

// WithErrorHandler installs a terminal error handler.
func WithErrorHandler[E any](h ErrorHandler) Option[E] {
	return func(cfg *Config[E]) {
		cfg.ErrorHandler = h
	}
}

// WithMiddleware appends global middleware.
func WithMiddleware[E any](m ...Middleware) Option[E] {
	return func(cfg *Config[E]) {
		cfg.Middleware.Use(m...)
	}
}

// WithEngine passes a framework-native engine into the factory.
func WithEngine[E any](engine E) Option[E] {
	return func(cfg *Config[E]) {
		cfg.Engine = engine
	}
}

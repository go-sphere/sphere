// Package requestid provides middleware that gives every request a correlation
// ID and carries it to the layers below and back to the client.
//
// The ID travels three ways: in the request's context.Context (read it with
// [FromContext]), in the httpx state store under [StoreKey], and on the
// response header ([DefaultHeader] unless changed). It is also attached to the
// context as a log attr named [LogKey] with log.ContextWithAttrs, so entries
// written through log.InfoContext and the logger middleware carry it.
//
// # Trust boundary
//
// By default an ID supplied by the client is accepted when it is at most
// [MaxLength] characters and made only of [A-Za-z0-9._-]; anything else is
// replaced by a freshly generated ID. Accepting client values lets a caller
// join a trace started upstream, but also lets any client choose the value that
// appears in your logs, so do not treat it as unique or authenticated. Use
// [WithAlwaysGenerate] at an edge that must not trust callers.
//
// Register the middleware before the logger middleware so the access log sees
// the ID, and before recovery so panic entries carry it.
package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/log"
)

const (
	// DefaultHeader is the request and response header carrying the ID.
	DefaultHeader = "X-Request-ID"
	// StoreKey is the httpx state-store key holding the ID as a string.
	StoreKey = "request.id"
	// LogKey is the attr key under which the ID is attached to log entries.
	LogKey = "request_id"
	// MaxLength is the longest client-supplied ID that is accepted.
	MaxLength = 64
)

type ctxKey struct{}

// FromContext returns the request ID stored by the middleware, or "" when ctx
// did not pass through it.
func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

type config struct {
	header         string
	alwaysGenerate bool
	generate       func() string
}

// Option configures [New].
type Option func(*config)

// WithHeader sets the header read from the request and written to the
// response. An empty name keeps the default.
func WithHeader(name string) Option {
	return func(c *config) {
		if name != "" {
			c.header = name
		}
	}
}

// WithAlwaysGenerate ignores any client-supplied ID and always mints a new one.
func WithAlwaysGenerate() Option {
	return func(c *config) { c.alwaysGenerate = true }
}

// WithGenerator replaces the ID generator. fn must be safe for concurrent use
// and return IDs that satisfy the accepted character set; nil keeps the default
// (32 hex characters from crypto/rand).
func WithGenerator(fn func() string) Option {
	return func(c *config) {
		if fn != nil {
			c.generate = fn
		}
	}
}

// New returns middleware that resolves the request ID (see the package comment
// for the trust rules), publishes it to the context, the state store and the
// response header, then calls the next handler. The header is set before the
// chain runs, so responses the service rejects carry it too.
func New(opts ...Option) httpx.Middleware {
	cfg := config{header: DefaultHeader, generate: generateID}
	for _, opt := range opts {
		opt(&cfg)
	}
	return func(next httpx.Handler) httpx.Handler {
		return func(ctx httpx.Context) error {
			id := ""
			if !cfg.alwaysGenerate {
				if in := ctx.Header(cfg.header); valid(in) {
					id = in
				}
			}
			if id == "" {
				id = cfg.generate()
			}
			ctx.Set(StoreKey, id)
			c := context.WithValue(ctx.Context(), ctxKey{}, id)
			ctx.SetContext(log.ContextWithAttrs(c, log.String(LogKey, id)))
			ctx.SetHeader(cfg.header, id)
			return next(ctx)
		}
	}
}

func valid(id string) bool {
	if id == "" || len(id) > MaxLength {
		return false
	}
	for i := range len(id) {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '.', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}

func generateID() string {
	var b [16]byte
	// rand.Read does not return an error on supported platforms (Go 1.24+).
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

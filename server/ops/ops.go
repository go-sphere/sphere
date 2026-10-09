// Package ops provides operational HTTP endpoints that must stay off the
// business listener: a liveness probe and, strictly opt-in, pprof.
//
// # Security
//
// pprof exposes heap contents, goroutine stacks, command line arguments and
// lets a caller trigger CPU profiles and traces, which is a denial-of-service
// and information-disclosure surface. Enable it only with [WithPprof], serve
// it only from [Handler] or [NewServer] on a listener that is not reachable
// from the public internet (loopback, a private interface, or behind network
// policy plus authentication), and never register it on the business engine or
// any mux that shares its listener. No function in this package mounts pprof
// on an httpx engine. [NewServer] does not authenticate; an empty or wildcard
// host in its address listens on every interface.
//
// Importing net/http/pprof also registers its handlers on
// http.DefaultServeMux as a side effect of that package. Nothing in sphere
// serves http.DefaultServeMux; do not serve it either.
//
// # Liveness only
//
// /healthz reports that the process is serving HTTP, nothing more. Readiness
// (database, cache or dependency checks) is deliberately out of scope.
package ops

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/pprof"
	"sync/atomic"
	"time"

	"github.com/go-sphere/sphere/core/task"
	"github.com/go-sphere/sphere/server/httpz"
)

// HealthzPath is the path of the liveness endpoint.
const HealthzPath = "/healthz"

// PprofPrefix is the path prefix of the pprof endpoints when enabled.
const PprofPrefix = "/debug/pprof/"

type config struct {
	pprof bool
}

// Option configures [Handler] and [NewServer].
type Option func(*config)

// WithPprof serves the net/http/pprof endpoints under [PprofPrefix]. It is off
// by default; read the package security notes before using it.
func WithPprof() Option {
	return func(c *config) { c.pprof = true }
}

// HealthzHandler returns the liveness handler: 200 with body "ok" for GET and
// HEAD, 405 with an Allow header otherwise. It is safe to mount on the
// business engine (httpz.MountStdAll or httpx.MountStd) when a probe should
// share that listener; it exposes nothing but the fact that the process is up.
func HealthzHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte("ok"))
		}
	})
}

// Handler returns a new, independent http.ServeMux serving [HealthzPath] and,
// with [WithPprof], the pprof endpoints. It never touches
// http.DefaultServeMux. Serve it on a dedicated listener; see the package
// security notes.
func Handler(opts ...Option) http.Handler {
	var c config
	for _, opt := range opts {
		opt(&c)
	}
	mux := http.NewServeMux()
	mux.Handle(HealthzPath, HealthzHandler())
	if c.pprof {
		mux.HandleFunc(PprofPrefix, pprof.Index)
		mux.HandleFunc(PprofPrefix+"cmdline", pprof.Cmdline)
		mux.HandleFunc(PprofPrefix+"profile", pprof.Profile)
		mux.HandleFunc(PprofPrefix+"symbol", pprof.Symbol)
		mux.HandleFunc(PprofPrefix+"trace", pprof.Trace)
	}
	return mux
}

// Server is a task.Task serving [Handler] on its own listener, separate from
// the business engine.
type Server struct {
	addr    string
	srv     *http.Server
	bound   atomic.Pointer[string]
	started atomic.Bool
}

var _ task.Task = (*Server)(nil)

// NewServer returns a Server that will listen on addr (host:port) and serve
// Handler(opts...). Prefer a loopback address such as "127.0.0.1:6060", above
// all with [WithPprof]. The server sets ReadHeaderTimeout but no write timeout,
// because pprof profiles stream for their requested duration.
func NewServer(addr string, opts ...Option) *Server {
	return &Server{
		addr: addr,
		srv: &http.Server{
			Handler:           Handler(opts...),
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
}

// Identifier returns "ops".
func (s *Server) Identifier() string { return "ops" }

// Addr returns the address the listener is bound to, or "" before Start has
// bound it. It differs from the configured address when that used port 0.
func (s *Server) Addr() string {
	if p := s.bound.Load(); p != nil {
		return *p
	}
	return ""
}

// Start binds the listener and serves until Stop. It blocks, and returns nil
// after a graceful Stop and the listen or serve error otherwise. A Server is
// single use: a second Start returns an error, and a Start after Stop returns
// nil as soon as the listener has been closed again.
func (s *Server) Start(_ context.Context) error {
	if s.started.Swap(true) {
		return errors.New("ops: server already started")
	}
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	a := ln.Addr().String()
	s.bound.Store(&a)
	if err := s.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Stop shuts the server down with httpz.StopServer: it drains in-flight
// requests until ctx expires, then force-closes. It is safe to call more
// than once and before Start.
func (s *Server) Stop(ctx context.Context) error {
	return httpz.StopServer(ctx, s.srv)
}

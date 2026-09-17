// Package docs is a task.Task HTTP server that serves an HTML index of
// Swagger targets, Swagger UI per spec, and reverse-proxies
// /{instanceName}/api to each target.
//
// CORS here echoes the request Origin with credentials (dev-oriented). That
// is the combination middleware/cors.NewCORS rejects. Identifier is "docs".
// Start after Stop returns nil without listening.
package docs

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/go-sphere/httpx"
	"github.com/go-sphere/sphere/server/httpz"
	httpSwagger "github.com/swaggo/http-swagger"
	"github.com/swaggo/swag"
)

// Target represents a documentation target with its address and Swagger specification.
type Target struct {
	Address string
	Spec    *swag.Spec
}

// Config contains the configuration for the documentation web service.
type Config struct {
	Address string
	Targets []Target
}

// Web provides a documentation web server that aggregates multiple Swagger specifications.
type Web struct {
	config  Config
	server  *http.Server
	engine  httpx.Engine
	mu      sync.Mutex
	stopped bool
}

// indexTarget is the subset of a Target the index page renders. Resolving it up
// front keeps the template from reaching into the caller's *swag.Spec.
type indexTarget struct {
	Name        string
	Description string
	Version     string
	Address     string
}

func (w *Web) newHandler() (http.Handler, error) {
	mux := http.NewServeMux()

	indexes := make([]indexTarget, 0, len(w.config.Targets))
	seen := make(map[string]string, len(w.config.Targets))
	for _, target := range w.config.Targets {
		if target.Spec == nil {
			return nil, fmt.Errorf("docs: target %s has no swagger spec", target.Address)
		}
		name, description := resolveTarget(target.Spec, target.Address)
		// Route prefixes are the lowercased instance name, so two targets that
		// differ only in case collide as well. http.ServeMux reports a collision
		// by panicking, which would turn a config mistake into a startup crash
		// out of a function that already returns an error.
		key := strings.ToLower(name)
		if previous, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("docs: targets %s and %s both use swagger instance name %q; their proxy and UI routes would collide", previous, target.Address, name)
		}
		seen[key] = target.Address
		indexes = append(indexes, indexTarget{
			Name:        name,
			Description: description,
			Version:     target.Spec.Version,
			Address:     target.Address,
		})
	}

	indexRaw, err := createIndex(indexes)
	if err != nil {
		return nil, err
	}
	mux.Handle("/", newIndexHandler(indexRaw))

	for _, target := range w.config.Targets {
		if err := registerTarget(mux, target.Spec, target.Address); err != nil {
			return nil, err
		}
	}

	return withCORS(mux), nil
}

// resolveTarget derives the values the docs server needs from a target: the
// instance name that drives its route prefix, and the description shown for it.
// Both are read-only, so the caller's *swag.Spec is left untouched.
func resolveTarget(spec *swag.Spec, address string) (name, description string) {
	name = spec.InstanceName()
	description = spec.Description
	if description == "" {
		description = fmt.Sprintf(" | proxy for %s", address)
	}
	return name, description
}

// NewWebServer creates a new documentation web server with the given configuration.
func NewWebServer(conf Config) *Web {
	return &Web{
		config: conf,
	}
}

// NewWebServerWithEngine serves the documentation on a caller-provided
// httpx.Engine instead of a private http.Server: routes are registered
// immediately via Register, and Start/Stop delegate to the engine, which
// keeps the engine's single-use lifecycle (Start after Stop returns
// httpx.ErrEngineClosed). conf.Address is ignored — the engine owns the
// listener. A nil engine panics: that is a programming error.
func NewWebServerWithEngine(conf Config, engine httpx.Engine) (*Web, error) {
	if engine == nil {
		panic("docs: NewWebServerWithEngine requires a non-nil engine")
	}
	w := &Web{
		config: conf,
		engine: engine,
	}
	if err := w.Register(engine.Group("")); err != nil {
		return nil, err
	}
	return w, nil
}

// Register mounts the documentation index, Swagger UI, and API reverse
// proxies (with the same relaxed CORS wrapper as the standalone server) onto
// any httpx.Registrar that supports mounting net/http handlers.
//
// The underlying http.ServeMux dispatches on the full request path, so r
// must be a root group; to mount under a prefix, wrap the registrar's routes
// with http.StripPrefix before calling Register. Only the catch-all
// "/*filepath" is registered — it also matches "/" on every official
// adapter, and registering "/" alongside it makes gin's router panic. The
// catch-all competes with other routes on the same engine (gin panics on
// conflicting wildcards) — prefer a dedicated engine or port for docs.
func (w *Web) Register(r httpx.Registrar) error {
	handler, err := w.newHandler()
	if err != nil {
		return err
	}
	return httpz.MountStdAll(r, "/*filepath", handler)
}

// Identifier returns the service identifier for the documentation web server.
func (w *Web) Identifier() string {
	return "docs"
}

// Start serves the documentation index, Swagger UI, and reverse-proxied APIs.
// In standalone mode a Start after Stop returns nil without listening; in
// engine mode (NewWebServerWithEngine) lifecycle is delegated to the engine,
// so a restart returns httpx.ErrEngineClosed instead.
func (w *Web) Start(ctx context.Context) error {
	if w.engine != nil {
		return w.engine.Start()
	}
	handler, err := w.newHandler()
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              w.config.Address,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	w.mu.Lock()
	if w.stopped {
		w.mu.Unlock()
		return nil
	}
	w.server = server
	w.mu.Unlock()
	return httpx.Start(server)
}

// Stop gracefully shuts down the documentation web server.
func (w *Web) Stop(ctx context.Context) error {
	if w.engine != nil {
		return w.engine.Stop(ctx)
	}
	w.mu.Lock()
	w.stopped = true
	server := w.server
	w.mu.Unlock()
	return httpz.StopServer(ctx, server)
}

func registerTarget(mux *http.ServeMux, spec *swag.Spec, target string) error {
	targetURL, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid target URL: %v", err)
	}
	// url.Parse accepts "localhost:8080" (scheme "localhost", empty host) and
	// bare paths; without this check the misconfiguration only surfaces as
	// per-request "unsupported protocol scheme" proxy failures.
	if (targetURL.Scheme != "http" && targetURL.Scheme != "https") || targetURL.Host == "" {
		return fmt.Errorf("invalid target URL %q: must be http(s)://host[:port]", target)
	}

	basePath := "/" + strings.ToLower(spec.InstanceName())

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxyPath := path.Join(basePath, "api")
	proxyHandler := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, proxyPath)
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		r.URL.RawPath = ""
		r.Host = targetURL.Host
		proxy.ServeHTTP(rw, r)
	})
	mux.Handle(proxyPath, proxyHandler)
	mux.Handle(proxyPath+"/", proxyHandler)

	docPath := path.Join(basePath, "doc", "swagger")
	name, description := resolveTarget(spec, target)
	swaggerHandler := httpSwagger.Handler(httpSwagger.InstanceName(name))
	mux.Handle(docPath, swaggerHandler)
	mux.Handle(docPath+"/", swaggerHandler)
	// More specific than the UI subtree above, so it wins for this path only.
	mux.Handle(docPath+"/doc.json", swaggerDocHandler(name, proxyPath, description))

	return nil
}

// swaggerDocHandler serves the target's Swagger document with the fields that
// only make sense behind this proxy rewritten on the way out: the UI is served
// by the docs server, so "Try it out" has to call back into /<instance>/api
// instead of the target's own host and base path.
//
// Rewriting the response rather than the spec is what keeps the caller's
// *swag.Spec read-only. That spec is usually also the one the generated docs
// package registered in the process-wide swag registry, so editing it would
// change what the API service's own /swagger/doc.json serves — and would race
// with the handler serving that document.
func swaggerDocHandler(instanceName, proxyPath, description string) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		raw, err := swag.ReadDoc(instanceName)
		if err != nil {
			http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		doc["host"] = ""
		doc["basePath"] = proxyPath
		if info, ok := doc["info"].(map[string]any); ok {
			if current, _ := info["description"].(string); current == "" {
				info["description"] = description
			}
		}
		body, err := json.Marshal(doc)
		if err != nil {
			http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = rw.Write(body)
	})
}

func newIndexHandler(body []byte) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(rw, r)
			return
		}
		rw.Header().Set("Content-Type", "text/html")
		rw.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = rw.Write(body)
		}
	})
}

// withCORS wraps the documentation handler with relaxed CORS headers for development/debugging.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// For local development and quick debugging of Swagger docs. Allow any
		// origin, but never with credentials: echoing the Origin together with
		// Access-Control-Allow-Credentials would let any website drive
		// cookie-bearing requests through this server to the proxied APIs.
		// Bearer tokens set explicitly by the caller (swagger "Authorize") do
		// not need credentialed CORS.
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		rw.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS, PUT, POST, DELETE, UPDATE")
		rw.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if r.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(rw, r)
	})
}

//go:embed index.tmpl
var indexHTML string

// createIndex generates an HTML index page listing all available documentation targets.
func createIndex(targets []indexTarget) ([]byte, error) {
	tmpl, err := template.New("index").Funcs(template.FuncMap{
		"lower": strings.ToLower,
	}).Parse(indexHTML)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, targets); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

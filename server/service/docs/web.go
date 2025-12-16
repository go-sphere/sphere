package docs

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"

	"github.com/go-sphere/httpx"
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
	config *Config
	server *http.Server
}

func (w *Web) newHandler() (http.Handler, error) {
	mux := http.NewServeMux()

	indexRaw, err := createIndex(w.config.Targets)
	if err != nil {
		return nil, err
	}
	mux.Handle("/", newIndexHandler(indexRaw))

	for _, spec := range w.config.Targets {
		if err := registerTarget(mux, spec.Spec, spec.Address); err != nil {
			return nil, err
		}
	}

	return withCORS(mux), nil
}

// NewWebServer creates a new documentation web server with the given configuration.
func NewWebServer(conf *Config) *Web {
	return &Web{
		config: conf,
	}
}

// Identifier returns the service identifier for the documentation web server.
func (w *Web) Identifier() string {
	return "docs"
}

// Start begins serving the documentation web server with Swagger UI for all configured targets.
// It sets up proxying to target services and provides a unified documentation interface.
func (w *Web) Start(ctx context.Context) error {
	handler, err := w.newHandler()
	if err != nil {
		return err
	}

	w.server = &http.Server{
		Addr:    w.config.Address,
		Handler: handler,
	}
	return httpx.Start(w.server)
}

// Stop gracefully shuts down the documentation web server.
func (w *Web) Stop(ctx context.Context) error {
	return httpx.Close(ctx, w.server)
}

func registerTarget(mux *http.ServeMux, spec *swag.Spec, target string) error {
	targetURL, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid target URL: %v", err)
	}

	basePath := "/" + strings.ToLower(spec.InstanceName())
	spec.Host = ""
	spec.BasePath = path.Join(basePath, "api")
	if spec.Description == "" {
		spec.Description = fmt.Sprintf(" | proxy for %s", target)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxyPath := spec.BasePath
	proxyHandler := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, proxyPath)
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		proxy.ServeHTTP(rw, r)
	})
	mux.Handle(proxyPath, proxyHandler)
	mux.Handle(proxyPath+"/", proxyHandler)

	docPath := path.Join(basePath, "doc", "swagger")
	swaggerHandler := httpSwagger.Handler(httpSwagger.InstanceName(spec.InstanceName()))
	mux.Handle(docPath, swaggerHandler)
	mux.Handle(docPath+"/", swaggerHandler)

	return nil
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

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		rw.Header().Set("Access-Control-Allow-Credentials", "true")
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
func createIndex(targets []Target) ([]byte, error) {
	tmpl, err := template.New("index").Funcs(template.FuncMap{
		"lower": strings.ToLower,
	}).Parse(indexHTML)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	_ = tmpl.Execute(&buf, targets)
	return buf.Bytes(), nil
}

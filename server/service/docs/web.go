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

	"github.com/gin-gonic/gin"
	"github.com/go-sphere/sphere/server/ginx"
	"github.com/go-sphere/sphere/server/middleware/cors"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
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
	engine := gin.Default()
	cors.Setup(engine, []string{"*"})

	for _, spec := range w.config.Targets {
		if err := setup(spec.Spec, engine, spec.Address); err != nil {
			return err
		}
	}
	indexRaw, err := createIndex(w.config.Targets)
	if err != nil {
		return err
	}
	engine.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html", indexRaw)
	})

	w.server = &http.Server{
		Addr:    w.config.Address,
		Handler: engine.Handler(),
	}
	return ginx.Start(w.server)
}

// Stop gracefully shuts down the documentation web server.
func (w *Web) Stop(ctx context.Context) error {
	return ginx.Close(ctx, w.server)
}

func setup(spec *swag.Spec, router gin.IRouter, target string) error {
	targetURL, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid target URL: %v", err)
	}

	route := router.Group("/" + strings.ToLower(spec.InstanceName()))

	spec.Host = ""
	spec.BasePath = path.Join(route.BasePath(), "api")
	if spec.Description == "" {
		spec.Description = fmt.Sprintf(" | proxy for %s", target)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	Setup(route.Group("/doc"), spec)
	route.Any("/api/*path", func(c *gin.Context) {
		c.Request.URL.Path = c.Param("path")
		proxy.ServeHTTP(c.Writer, c.Request)
	})

	return nil
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

// Setup registers Swagger UI endpoints for a given Swagger specification.
func Setup(route gin.IRoutes, doc *swag.Spec) {
	route.GET("/swagger/*any", ginSwagger.WrapHandler(
		swaggerFiles.NewHandler(),
		ginSwagger.InstanceName(doc.InstanceName()),
	))
}

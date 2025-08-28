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

type Target struct {
	Address string
	Spec    *swag.Spec
}

type Config struct {
	Address string
	Targets []Target
}

type Web struct {
	config *Config
	server *http.Server
}

func NewWebServer(conf *Config) *Web {
	return &Web{
		config: conf,
	}
}

func (w *Web) Identifier() string {
	return "docs"
}

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

func Setup(route gin.IRoutes, doc *swag.Spec) {
	route.GET("/swagger/*any", ginSwagger.WrapHandler(
		swaggerFiles.NewHandler(),
		ginSwagger.InstanceName(doc.InstanceName()),
	))
}

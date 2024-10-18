package docs

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/swaggo/swag"
	"github.com/tbxark/sphere/docs/api"
	"github.com/tbxark/sphere/docs/dash"
	"github.com/tbxark/sphere/pkg/web/route/docs"
	"net/http/httputil"
	"net/url"
)

type Targets struct {
	API  string `json:"api" yaml:"api"`
	Dash string `json:"dash" yaml:"dash"`
}

type Config struct {
	Address string  `json:"address" yaml:"address"`
	Targets Targets `json:"targets" yaml:"targets"`
}

type Web struct {
	config *Config
	engine *gin.Engine
}

func NewWebServer(conf *Config) *Web {
	return &Web{
		config: conf,
		engine: gin.Default(),
	}
}

func (w *Web) Identifier() string {
	return "docs"
}

func (w *Web) Run() error {
	err := setup(api.SwaggerInfoAPI, w.engine, "api", w.config.Targets.API)
	if err != nil {
		return err
	}
	err = setup(dash.SwaggerInfoDash, w.engine, "dash", w.config.Targets.Dash)
	if err != nil {
		return err
	}
	return w.engine.Run(w.config.Address)
}

func setup(spec *swag.Spec, engine *gin.Engine, group, target string) error {

	spec.Host = ""
	spec.BasePath = fmt.Sprintf("/%s/api", group)

	route := engine.Group("/" + group)
	docs.Setup(route.Group("/doc"), spec)
	targetURL, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid target URL: %v", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	route.Any("/api/*path", func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Request.URL.Path = c.Param("path")
		proxy.ServeHTTP(c.Writer, c.Request)
	})
	return nil
}

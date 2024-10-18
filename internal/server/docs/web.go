package docs

import (
	"github.com/gin-gonic/gin"
	"github.com/tbxark/sphere/docs/api"
	"github.com/tbxark/sphere/docs/dash"
	"github.com/tbxark/sphere/pkg/web/route/docs"
)

type Hosts struct {
	API  string `json:"api" yaml:"api"`
	Dash string `json:"dash" yaml:"dash"`
}

type Config struct {
	Address string `json:"address" yaml:"address"`
	Hosts   Hosts  `json:"hosts" yaml:"hosts"`
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
	api.SwaggerInfoAPI.Host = w.config.Hosts.API
	docs.Setup(w.engine.Group("/api"), api.SwaggerInfoAPI)
	dash.SwaggerInfoDash.Host = w.config.Hosts.Dash
	docs.Setup(w.engine.Group("/dash"), dash.SwaggerInfoDash)
	return w.engine.Run(w.config.Address)
}

package docs

import (
	"github.com/TBXark/sphere/example/swagger/api"
	"github.com/TBXark/sphere/example/swagger/dash"
	"github.com/TBXark/sphere/server/service/docs"
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
	*docs.Web
}

func NewWebServer(config *Config) *Web {
	return &Web{
		Web: docs.NewWebServer(&docs.Config{
			Address: config.Address,
			Targets: []docs.Target{
				{Address: config.Targets.API, Spec: api.SwaggerInfoAPI},
				{Address: config.Targets.Dash, Spec: dash.SwaggerInfoDash},
			},
		}),
	}
}

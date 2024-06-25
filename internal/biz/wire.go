package biz

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/internal/biz/api"
	"github.com/tbxark/go-base-api/internal/biz/dash"
	"github.com/tbxark/go-base-api/internal/biz/render"
)

var ProviderSet = wire.NewSet(api.NewWebServer, dash.NewWebServer, render.NewRender)

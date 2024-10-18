package server

import (
	"github.com/google/wire"
	"github.com/tbxark/sphere/internal/server/api"
	"github.com/tbxark/sphere/internal/server/dash"
	"github.com/tbxark/sphere/internal/server/docs"
)

var ProviderSet = wire.NewSet(api.NewWebServer, dash.NewWebServer, docs.NewWebServer)

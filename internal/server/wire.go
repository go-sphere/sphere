package server

import (
	"github.com/google/wire"
	"github.com/tbxark/sphere/internal/server/api"
	"github.com/tbxark/sphere/internal/server/dash"
)

var ProviderSet = wire.NewSet(api.NewWebServer, dash.NewWebServer)

package server

import (
	"github.com/google/wire"
	"github.com/tbxark/go-base-api/internal/server/api"
	"github.com/tbxark/go-base-api/internal/server/dash"
)

var ProviderSet = wire.NewSet(api.NewWebServer, dash.NewWebServer)

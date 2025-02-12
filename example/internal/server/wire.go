package server

import (
	"github.com/TBXark/sphere/example/internal/server/api"
	"github.com/TBXark/sphere/example/internal/server/bot"
	"github.com/TBXark/sphere/example/internal/server/dash"
	"github.com/TBXark/sphere/example/internal/server/docs"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(api.NewWebServer, dash.NewWebServer, docs.NewWebServer, bot.NewApp)

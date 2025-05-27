package server

import (
	"github.com/TBXark/sphere/layout/internal/server/api"
	"github.com/TBXark/sphere/layout/internal/server/bot"
	"github.com/TBXark/sphere/layout/internal/server/dash"
	"github.com/TBXark/sphere/layout/internal/server/docs"
	"github.com/TBXark/sphere/layout/internal/server/file"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	api.NewWebServer,
	dash.NewWebServer,
	docs.NewWebServer,
	file.NewWebServer,
	bot.NewApp,
)
